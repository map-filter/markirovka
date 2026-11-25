<?php

namespace App\Services;

use App\Ismpt\IsmptException;
use App\Ismpt\IsmptSession;
use App\Ismpt\TrueApiClient;
use App\Jobs\ProcessCodeOrder;
use App\Models\Code;
use App\Models\CodeOrder;
use App\Models\Operation;
use App\Models\Product;
use App\Models\User;
use App\Support\Gs1;
use Illuminate\Support\Carbon;
use Illuminate\Support\Facades\Cache;
use Illuminate\Support\Facades\DB;
use Throwable;

class CodeOrderService
{
    private const MAX_POLLS = 120;

    public function __construct(private readonly IsmptSession $ismpt) {}

    public function create(User $user, Product $product, int $quantity): CodeOrder
    {
        $org = $user->organization;
        $this->ismpt->token($org);
        if ($product->nkt_status !== Product::NKT_FOUND) {
            throw new IsmptException('Товара нет в НКТ: сначала проверьте или зарегистрируйте карточку', 409);
        }

        $order = DB::transaction(function () use ($user, $org, $product, $quantity) {
            DB::table('organizations')->where('id', $org->id)->lockForUpdate()->first();

            return CodeOrder::create([
                'organization_id' => $org->id,
                'user_id' => $user->id,
                'product_id' => $product->id,
                'number' => CodeOrder::nextNumber($org->id),
                'quantity' => $quantity,
                'status' => CodeOrder::NEW,
            ]);
        });
        Operation::log($org->id, 'order.created', "Заказ {$order->number}: {$quantity} кодов для «{$product->name}»", subject: $order, userId: $user->id);
        ProcessCodeOrder::dispatch($order->id);

        return $order;
    }

    public function process(CodeOrder $order): bool
    {
        $lock = Cache::lock("code-order:{$order->id}", 30);
        if (! $lock->get()) {
            return true;
        }
        try {
            return $this->step($order->refresh());
        } finally {
            $lock->release();
        }
    }

    private function step(CodeOrder $order): bool
    {
        if (! in_array($order->status, CodeOrder::ACTIVE, true)) {
            return false;
        }
        $org = $order->organization;

        try {
            if ($order->ismpt_order_id === null) {
                $uuid = $this->ismpt->call($org, fn (TrueApiClient $c, string $t) => $c->orderCodes($t, $order->product->gtin, $order->quantity));
                $order->update(['ismpt_order_id' => $uuid, 'status' => CodeOrder::SENT, 'sent_at' => now()]);
                Operation::log($org->id, 'order.sent', "Заказ {$order->number} отправлен в ИС МПТ", subject: $order, userId: null);

                return true;
            }

            $status = $this->ismpt->call($org, fn (TrueApiClient $c, string $t) => $c->orderStatus($t, $order->ismpt_order_id));
            $order->increment('attempts');
            if (in_array($status, ['VALIDATION_FAILED', 'ERROR', 'RESULT_EXPIRED'], true)) {
                return $this->fail($order, "ИС МПТ отклонила заказ: {$status}");
            }
            if ($status !== 'SUCCESS') {
                $order->update(['status' => CodeOrder::PROCESSING]);

                return $order->attempts < self::MAX_POLLS || $this->fail($order, 'ИС МПТ слишком долго обрабатывает заказ');
            }

            $items = $this->ismpt->call($org, fn (TrueApiClient $c, string $t) => $c->orderResult($t, $order->ismpt_order_id));
            $this->storeCodes($order, $items);

            return false;
        } catch (IsmptException $e) {
            if ($e->isUnauthorized() || $e->status >= 500) {
                $order->update(['error' => $e->getMessage()]);

                return $e->status >= 500 && $order->attempts < self::MAX_POLLS;
            }

            return $this->fail($order, $e->getMessage());
        }
    }

    /**
     * @param  list<array<string, mixed>>  $items
     */
    public function storeCodes(CodeOrder $order, array $items): void
    {
        $annulDays = (int) config('ismpt.annul_days');
        $now = now();
        $rows = [];
        $earliest = null;
        foreach ($items as $item) {
            $raw = (string) ($item['printCode'] ?? $item['code'] ?? $item['cis'] ?? '');
            try {
                $parsed = Gs1::parse($raw);
            } catch (Throwable) {
                continue;
            }
            $full = str_starts_with($raw, '(') ? Gs1::fromHumanReadable($raw) : $raw;
            $emitted = isset($item['emissionDate']) ? Carbon::parse($item['emissionDate']) : $now;
            $expires = $emitted->copy()->addDays($annulDays);
            $earliest = $earliest === null || $expires->lt($earliest) ? $expires : $earliest;
            $rows[] = [
                'organization_id' => $order->organization_id,
                'product_id' => $order->product_id,
                'code_order_id' => $order->id,
                'code' => $full,
                'cis' => $parsed['cis'],
                'gtin' => $parsed['gtin'],
                'serial' => $parsed['serial'],
                'status' => Code::RECEIVED,
                'emitted_at' => $emitted,
                'expires_at' => $expires,
                'created_at' => $now,
                'updated_at' => $now,
            ];
        }

        DB::transaction(function () use ($order, $rows, $earliest, $now) {
            foreach (array_chunk($rows, 500) as $chunk) {
                Code::insertOrIgnore($chunk);
            }
            $order->update([
                'status' => CodeOrder::READY,
                'ready_at' => $now,
                'expires_at' => $earliest,
                'error' => null,
            ]);
        });
        Operation::log($order->organization_id, 'order.ready', "Заказ {$order->number}: получено ".count($rows).' кодов, можно печатать', subject: $order, userId: null);
    }

    private function fail(CodeOrder $order, string $message): bool
    {
        $order->update(['status' => CodeOrder::FAILED, 'error' => $message]);
        Operation::log($order->organization_id, 'order.failed', "Заказ {$order->number}: {$message}", 'error', $order, userId: null);

        return false;
    }
}
