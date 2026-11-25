<?php

namespace App\Services;

use App\Ismpt\IsmptException;
use App\Ismpt\IsmptSession;
use App\Ismpt\TrueApiClient;
use App\Models\Code;
use App\Models\CodeOrder;
use App\Models\Operation;
use App\Models\Organization;
use Illuminate\Support\Carbon;

class CodeLifecycle
{
    public function __construct(private readonly IsmptSession $ismpt) {}

    public function annulExpired(Organization $org): int
    {
        $now = now();
        $count = Code::where('organization_id', $org->id)
            ->whereIn('status', [Code::RECEIVED, Code::PRINTED])
            ->where('expires_at', '<', $now)
            ->update(['status' => Code::ANNULLED, 'annulled_at' => $now, 'updated_at' => $now]);
        if ($count) {
            Operation::log($org->id, 'codes.annulled', "Аннулировано {$count} кодов: истёк срок без отчёта о нанесении", 'error', userId: null);
        }

        return $count;
    }

    public function syncWithIsmpt(Organization $org): int
    {
        if (! $org->hasIsmptSession()) {
            return 0;
        }
        $changed = 0;
        Code::where('organization_id', $org->id)
            ->whereIn('status', [Code::RECEIVED, Code::PRINTED])
            ->orderBy('id')
            ->chunkById(1000, function ($codes) use ($org, &$changed) {
                try {
                    $info = $this->ismpt->call($org, fn (TrueApiClient $c, string $t) => $c->cisesInfo($t, $codes->pluck('cis')->all()));
                } catch (IsmptException) {
                    return false;
                }
                $byCis = $codes->keyBy(fn (Code $c) => $c->cis);
                foreach ($info as $row) {
                    $cis = $row['cisInfo'] ?? [];
                    if (($cis['statusExt'] ?? null) !== 'ANNULLED') {
                        continue;
                    }
                    $key = preg_replace('/[()]/', '', (string) ($cis['cis'] ?? ''));
                    $code = $byCis[$key] ?? null;
                    if ($code) {
                        $code->update(['status' => Code::ANNULLED, 'annulled_at' => now()]);
                        $changed++;
                    }
                }

                return true;
            });
        if ($changed) {
            Operation::log($org->id, 'codes.annulled', "ИС МПТ аннулировала {$changed} кодов без отчёта о нанесении", 'error', userId: null);
        }

        return $changed;
    }

    public function remindExpiring(Organization $org): int
    {
        $days = (int) config('ismpt.remind_days');
        $rows = Code::where('organization_id', $org->id)
            ->whereIn('status', [Code::RECEIVED, Code::PRINTED])
            ->whereBetween('expires_at', [now(), now()->addDays($days)])
            ->selectRaw('code_order_id, min(expires_at) as expires_at, count(*) as total')
            ->groupBy('code_order_id')
            ->get();
        $sent = 0;
        foreach ($rows as $row) {
            $order = CodeOrder::find($row->code_order_id);
            $already = Operation::where('organization_id', $org->id)
                ->where('type', 'codes.expiring')
                ->where('subject_type', 'CodeOrder')
                ->where('subject_id', $order->id)
                ->where('created_at', '>', now()->subHours(20))
                ->exists();
            if ($already) {
                continue;
            }
            $left = max(0, (int) ceil(now()->diffInHours(Carbon::parse($row->expires_at), false) / 24));
            Operation::log($org->id, 'codes.expiring', "Заказ {$order->number}: {$row->total} кодов аннулируются через {$left} дн., если не передать отчёт о нанесении", 'warning', $order, userId: null);
            $sent++;
        }

        return $sent;
    }
}
