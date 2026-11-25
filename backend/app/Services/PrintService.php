<?php

namespace App\Services;

use App\Labels\LabelItem;
use App\Labels\LabelRenderer;
use App\Models\Code;
use App\Models\LabelTemplate;
use App\Models\Operation;
use App\Models\PrintAgent;
use App\Models\PrintJob;
use App\Models\User;
use Illuminate\Support\Collection;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Facades\Storage;
use Illuminate\Validation\ValidationException;

class PrintService
{
    public function __construct(private readonly LabelRenderer $renderer) {}

    /**
     * @param  array{template_id: int, format: string, target: string, agent_id?: ?int, code_ids?: list<int>, product_id?: ?int, order_id?: ?int, quantity?: ?int, reprint?: bool}  $data
     */
    public function create(User $user, array $data): PrintJob
    {
        $org = $user->organization;
        $template = LabelTemplate::where('organization_id', $org->id)->findOrFail($data['template_id']);
        $codes = $this->selectCodes($org->id, $data);
        if ($codes->isEmpty()) {
            throw ValidationException::withMessages(['code_ids' => 'Нет кодов для печати: все выбранные уже напечатаны или не получены']);
        }

        $agent = null;
        if ($data['target'] === 'agent') {
            $agent = isset($data['agent_id'])
                ? PrintAgent::where('organization_id', $org->id)->findOrFail($data['agent_id'])
                : PrintAgent::where('organization_id', $org->id)->orderByDesc('last_seen_at')->first();
            if (! $agent) {
                throw ValidationException::withMessages(['agent_id' => 'Агент печати не подключён: добавьте его в настройках']);
            }
        }

        return DB::transaction(function () use ($user, $org, $template, $codes, $data, $agent) {
            $job = PrintJob::create([
                'organization_id' => $org->id,
                'user_id' => $user->id,
                'label_template_id' => $template->id,
                'print_agent_id' => $agent?->id,
                'format' => $data['format'],
                'target' => $data['target'],
                'status' => $data['target'] === 'agent' ? PrintJob::QUEUED : PrintJob::READY,
                'codes_count' => $codes->count(),
            ]);
            $job->codes()->attach($codes->pluck('id'));

            $content = $this->render($template, $codes, $data['format'], $org->name.', '.$org->address);
            $path = "print-jobs/{$job->id}.{$data['format']}";
            Storage::disk('local')->put($path, $content);
            $job->update(['file_path' => $path]);

            if ($data['target'] === 'download') {
                $this->markPrinted($job);
            }
            $where = $agent ? "агенту «{$agent->name}»" : 'для скачивания';
            Operation::log($org->id, 'print.created', "Печать {$codes->count()} этикеток (".strtoupper($data['format']).", {$template->size}) {$where}", subject: $job, userId: $user->id);

            return $job;
        });
    }

    /**
     * @param  Collection<int, Code>  $codes
     */
    public function render(LabelTemplate $template, Collection $codes, string $format, string $manufacturer): string
    {
        $items = $codes->map(fn (Code $code) => new LabelItem(
            $code->code,
            $code->product->name,
            $code->product->size,
            $code->product->composition,
            $code->gtin,
            $manufacturer,
        ));

        return $format === 'zpl'
            ? $this->renderer->zpl($template, $items)
            : $this->renderer->pdf($template, $items);
    }

    public function markPrinted(PrintJob $job): void
    {
        $now = now();
        Code::whereIn('id', $job->codes()->pluck('codes.id'))
            ->where('status', Code::RECEIVED)
            ->update(['status' => Code::PRINTED, 'printed_at' => $now, 'updated_at' => $now]);
        $job->update(['status' => PrintJob::PRINTED, 'printed_at' => $now]);
    }

    /**
     * @return Collection<int, Code>
     */
    private function selectCodes(int $orgId, array $data): Collection
    {
        $statuses = ($data['reprint'] ?? false) ? [Code::RECEIVED, Code::PRINTED] : [Code::RECEIVED];
        $query = Code::with('product')->where('organization_id', $orgId)->whereIn('status', $statuses);

        if (! empty($data['code_ids'])) {
            $query->whereIn('id', $data['code_ids']);
        } elseif (! empty($data['order_id'])) {
            $query->where('code_order_id', $data['order_id']);
        } elseif (! empty($data['product_id'])) {
            $query->where('product_id', $data['product_id']);
        } else {
            throw ValidationException::withMessages(['code_ids' => 'Выберите коды, товар или заказ']);
        }

        $limit = empty($data['code_ids']) && ! empty($data['quantity']) ? min((int) $data['quantity'], 5000) : 5000;

        return $query->orderBy('expires_at')->orderBy('id')->limit($limit)->get();
    }
}
