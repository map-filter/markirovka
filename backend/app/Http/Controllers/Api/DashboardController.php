<?php

namespace App\Http\Controllers\Api;

use App\Http\Controllers\Controller;
use App\Http\Resources\OperationResource;
use App\Ismpt\IsmptSession;
use App\Models\Code;
use App\Models\CodeOrder;
use App\Models\Document;
use App\Models\Operation;
use App\Models\Organization;
use App\Models\Product;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;

class DashboardController extends Controller
{
    public function __invoke(Request $request, IsmptSession $ismpt): JsonResponse
    {
        $org = $request->user()->organization;
        $counts = array_merge(Code::emptyCounts(), Code::where('organization_id', $org->id)
            ->selectRaw('status, count(*) as total')->groupBy('status')->pluck('total', 'status')->map(fn ($v) => (int) $v)->all());
        $expiring = $this->expiring($org);

        return response()->json([
            'ismpt' => $ismpt->status($org),
            'stats' => [
                'products' => Product::where('organization_id', $org->id)->count(),
                'products_in_nkt' => Product::where('organization_id', $org->id)->where('nkt_status', Product::NKT_FOUND)->count(),
                'orders_in_progress' => CodeOrder::where('organization_id', $org->id)->whereIn('status', CodeOrder::ACTIVE)->count(),
                'codes' => $counts,
            ],
            'todo' => $this->todo($org, $counts, $expiring),
            'expiring' => $expiring,
            'recent_operations' => OperationResource::collection(
                Operation::with('user')->where('organization_id', $org->id)->latest('id')->limit(8)->get()
            ),
        ]);
    }

    private function expiring(Organization $org): array
    {
        $rows = Code::where('organization_id', $org->id)
            ->whereIn('status', [Code::RECEIVED, Code::PRINTED])
            ->where('expires_at', '<=', now()->addDays(config('ismpt.remind_days')))
            ->selectRaw('code_order_id, min(expires_at) as expires_at, count(*) as total')
            ->groupBy('code_order_id')
            ->orderBy('expires_at')
            ->get();
        $orders = CodeOrder::with('product')->whereIn('id', $rows->pluck('code_order_id'))->get()->keyBy('id');

        return $rows->map(function ($row) use ($orders) {
            $order = $orders[$row->code_order_id];
            $expires = \Illuminate\Support\Carbon::parse($row->expires_at);

            return [
                'product' => ['id' => $order->product->id, 'gtin' => $order->product->gtin, 'name' => $order->product->name],
                'order_id' => $order->id,
                'order_number' => $order->number,
                'count' => (int) $row->total,
                'expires_at' => $expires->toIso8601String(),
                'days_left' => max(0, (int) ceil(now()->diffInHours($expires, false) / 24)),
            ];
        })->all();
    }

    private function todo(Organization $org, array $counts, array $expiring): array
    {
        $todo = [];
        if (! $org->hasIsmptSession()) {
            $todo[] = $this->item('connect_ismpt', 'warning', 'Подключитесь к ИС МПТ', 'Вход по ЭЦП через NCALayer, сессия действует 10 часов', null, 'Подключить', 'dashboard');
        }
        foreach ($expiring as $row) {
            $todo[] = $this->item('expiring_codes', 'danger', "Коды аннулируются через {$row['days_left']} дн.", "Заказ {$row['order_number']}, {$row['product']['name']}: передайте отчёт о нанесении", $row['count'], 'Нанести', 'circulation', ['order_id' => $row['order_id']]);
        }
        $failed = CodeOrder::where('organization_id', $org->id)->where('status', CodeOrder::FAILED)->where('updated_at', '>', now()->subDays(3))->count();
        if ($failed) {
            $todo[] = $this->item('order_failed', 'danger', 'Заказы кодов с ошибкой', 'Проверьте причину и закажите снова', $failed, 'Открыть', 'orders', ['status' => 'failed']);
        }
        $rejected = Document::where('organization_id', $org->id)->where('status', Document::REJECTED)->where('updated_at', '>', now()->subDays(3))->count();
        if ($rejected) {
            $todo[] = $this->item('document_rejected', 'danger', 'Документы отклонены ИС МПТ', 'Посмотрите ошибки по кодам', $rejected, 'Открыть', 'circulation');
        }
        $missing = Product::where('organization_id', $org->id)->where('nkt_status', '!=', Product::NKT_FOUND)->count();
        if ($missing) {
            $todo[] = $this->item('nkt_missing', 'warning', 'Товары без карточки в НКТ', 'Без карточки коды не заказать', $missing, 'К товарам', 'products');
        }
        if ($counts[Code::RECEIVED] > 0) {
            $todo[] = $this->item('print_codes', 'info', 'Напечатайте этикетки', 'Коды получены, но ещё не напечатаны', $counts[Code::RECEIVED], 'Печать', 'print');
        }
        if ($counts[Code::PRINTED] > 0) {
            $todo[] = $this->item('apply_codes', 'info', 'Передайте отчёт о нанесении', 'Этикетки напечатаны: после нанесения на изделия отправьте отчёт', $counts[Code::PRINTED], 'Нанести', 'circulation');
        }
        if ($counts[Code::APPLIED] > 0) {
            $todo[] = $this->item('introduce_codes', 'info', 'Введите товар в оборот', 'Коды нанесены, товар можно вводить в оборот', $counts[Code::APPLIED], 'Ввести', 'circulation');
        }
        $ready = CodeOrder::where('organization_id', $org->id)->where('status', CodeOrder::READY)->where('ready_at', '>', now()->subDay())->count();
        if ($ready) {
            $todo[] = $this->item('order_ready', 'info', 'Заказы кодов готовы', 'За последние сутки', $ready, 'Заказы', 'orders', ['status' => 'ready']);
        }

        return $todo;
    }

    private function item(string $kind, string $severity, string $title, string $description, ?int $count, string $label, string $route, array $query = []): array
    {
        return [
            'kind' => $kind,
            'severity' => $severity,
            'title' => $title,
            'description' => $description,
            'count' => $count,
            'action' => ['label' => $label, 'route' => $route, 'query' => (object) $query],
        ];
    }
}
