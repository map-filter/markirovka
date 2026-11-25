<?php

namespace App\Http\Controllers\Api;

use App\Http\Controllers\Controller;
use App\Models\Code;
use App\Models\CodeOrder;
use App\Models\Organization;
use App\Support\Gs1;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;
use Illuminate\Validation\Rule;
use Symfony\Component\HttpFoundation\Response;
use Throwable;

class IntegrationController extends Controller
{
    public function codes(Request $request): Response
    {
        $org = $this->organization($request);
        $request->validate([
            'format' => ['nullable', 'in:json,csv'],
            'status' => ['nullable', Rule::in(Code::STATUSES)],
            'updated_since' => ['nullable', 'date'],
        ]);
        $query = Code::with(['product', 'codeOrder'])->where('organization_id', $org->id)->orderBy('id');
        if ($number = $request->query('order')) {
            $order = CodeOrder::where('organization_id', $org->id)->where('number', $number)->first();
            $query->where('code_order_id', $order?->id ?? 0);
        }
        if ($gtin = $request->query('gtin')) {
            $query->where('gtin', Gs1::normalizeGtin($gtin));
        }
        if ($status = $request->query('status')) {
            $query->where('status', $status);
        }
        if ($since = $request->query('updated_since')) {
            $query->where('updated_at', '>=', $since);
        }
        $codes = $query->limit(20000)->get()->map(fn (Code $c) => $this->present($c));

        if ($request->query('format') === 'csv') {
            return response()->streamDownload(function () use ($codes) {
                $out = fopen('php://output', 'w');
                fwrite($out, "\xEF\xBB\xBF");
                fputcsv($out, ['code_base64', 'code_hr', 'gtin', 'serial', 'status', 'status_label', 'product', 'order', 'updated_at'], ';');
                foreach ($codes as $c) {
                    fputcsv($out, [$c['code_base64'], $c['code_hr'], $c['gtin'], $c['serial'], $c['status'], $c['status_label'], $c['product'], $c['order'], $c['updated_at']], ';');
                }
                fclose($out);
            }, 'kody.csv', ['Content-Type' => 'text/csv; charset=UTF-8']);
        }

        return response()->json([
            'organization' => $org->name,
            'bin' => $org->bin,
            'generated_at' => now()->toIso8601String(),
            'codes' => $codes,
        ]);
    }

    public function statuses(Request $request): JsonResponse
    {
        $org = $this->organization($request);
        $data = $request->validate([
            'codes' => ['required', 'array', 'max:5000'],
            'codes.*' => ['string'],
        ]);
        $byCis = [];
        $notFound = [];
        foreach ($data['codes'] as $raw) {
            try {
                $byCis[Gs1::parse($raw)['cis']] = $raw;
            } catch (Throwable) {
                $notFound[] = $raw;
            }
        }
        $codes = Code::with(['product', 'codeOrder'])
            ->where('organization_id', $org->id)
            ->whereIn('cis', array_keys($byCis))
            ->get()
            ->keyBy('cis');
        foreach ($byCis as $cis => $raw) {
            if (! $codes->has($cis)) {
                $notFound[] = $raw;
            }
        }

        return response()->json([
            'codes' => $codes->values()->map(fn (Code $c) => $this->present($c)),
            'not_found' => $notFound,
        ]);
    }

    public function orders(Request $request): JsonResponse
    {
        $org = $this->organization($request);
        $orders = CodeOrder::with('product')->where('organization_id', $org->id)->latest('id')->limit(200)->get();

        return response()->json([
            'orders' => $orders->map(fn (CodeOrder $o) => [
                'number' => $o->number,
                'gtin' => $o->product->gtin,
                'product' => $o->product->name,
                'quantity' => $o->quantity,
                'status' => $o->status,
                'created_at' => $o->created_at?->toIso8601String(),
            ]),
        ]);
    }

    private function present(Code $code): array
    {
        return [
            'code' => $code->code,
            'code_base64' => base64_encode($code->code),
            'code_hr' => Gs1::humanReadable($code->code),
            'gtin' => $code->gtin,
            'serial' => $code->serial,
            'status' => $code->status,
            'status_label' => Code::LABELS[$code->status] ?? $code->status,
            'product' => $code->product?->name,
            'order' => $code->codeOrder?->number,
            'updated_at' => $code->updated_at?->toIso8601String(),
        ];
    }

    private function organization(Request $request): Organization
    {
        $token = $request->bearerToken();
        $org = $token ? Organization::where('integration_token_hash', hash('sha256', $token))->first() : null;
        abort_unless($org, 401, 'Неверный токен интеграции');

        return $org;
    }
}
