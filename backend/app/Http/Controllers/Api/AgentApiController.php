<?php

namespace App\Http\Controllers\Api;

use App\Http\Controllers\Controller;
use App\Models\Operation;
use App\Models\PrintAgent;
use App\Models\PrintJob;
use App\Services\PrintService;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;
use Illuminate\Http\Response;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Facades\Storage;

class AgentApiController extends Controller
{
    public function next(Request $request): JsonResponse|Response
    {
        $agent = $this->agent($request);
        $wait = max(0, min((int) $request->query('wait', 20), (int) config('ismpt.agent_max_wait')));
        $deadline = microtime(true) + $wait;
        $agent->forceFill(['last_seen_at' => now()])->save();

        do {
            if ($job = $this->claim($agent)) {
                return response()->json([
                    'id' => (string) $job->id,
                    'format' => $job->format,
                    'copies' => 1,
                    'printer' => '',
                    'payload_base64' => base64_encode(Storage::disk('local')->get($job->file_path)),
                ]);
            }
            if (microtime(true) >= $deadline) {
                break;
            }
            usleep(1_000_000);
        } while (true);

        return response()->noContent();
    }

    public function ack(Request $request, string $id, PrintService $print): Response
    {
        $agent = $this->agent($request);
        $data = $request->validate([
            'status' => ['required', 'in:printed,failed'],
            'error' => ['nullable', 'string', 'max:2000'],
        ]);
        $job = PrintJob::where('print_agent_id', $agent->id)->findOrFail((int) $id);
        if (in_array($job->status, [PrintJob::PRINTED, PrintJob::FAILED], true)) {
            return response()->noContent();
        }
        if ($data['status'] === 'printed') {
            $print->markPrinted($job);
            Operation::log($job->organization_id, 'print.printed', "Агент «{$agent->name}» напечатал {$job->codes_count} этикеток", subject: $job, userId: $job->user_id);
        } else {
            $job->update(['status' => PrintJob::FAILED, 'error' => $data['error'] ?? 'Ошибка печати']);
            Operation::log($job->organization_id, 'print.failed', "Агент «{$agent->name}»: ошибка печати: ".($data['error'] ?? ''), 'error', $job, userId: $job->user_id);
        }

        return response()->noContent();
    }

    public function heartbeat(Request $request): Response
    {
        $agent = $this->agent($request);
        $data = $request->validate([
            'version' => ['nullable', 'string', 'max:50'],
            'hostname' => ['nullable', 'string', 'max:255'],
            'printer' => ['nullable', 'string', 'max:255'],
        ]);
        $agent->forceFill($data + ['last_seen_at' => now()])->save();

        return response()->noContent();
    }

    private function claim(PrintAgent $agent): ?PrintJob
    {
        return DB::transaction(function () use ($agent) {
            $job = PrintJob::where('print_agent_id', $agent->id)
                ->where(fn ($q) => $q->where('status', PrintJob::QUEUED)
                    ->orWhere(fn ($q) => $q->where('status', PrintJob::SENT)->where('sent_at', '<', now()->subMinutes(5))))
                ->orderBy('id')
                ->lockForUpdate()
                ->first();
            $job?->update(['status' => PrintJob::SENT, 'sent_at' => now()]);

            return $job;
        });
    }

    private function agent(Request $request): PrintAgent
    {
        $token = $request->bearerToken();
        $agent = $token ? PrintAgent::where('token_hash', PrintAgent::hashToken($token))->first() : null;
        abort_unless($agent, 401, 'Неверный токен агента');

        return $agent;
    }
}
