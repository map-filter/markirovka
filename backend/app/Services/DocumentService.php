<?php

namespace App\Services;

use App\Ismpt\IsmptException;
use App\Ismpt\IsmptSession;
use App\Ismpt\TrueApiClient;
use App\Jobs\ProcessDocument;
use App\Models\Code;
use App\Models\Document;
use App\Models\Operation;
use App\Models\User;
use Illuminate\Support\Collection;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Str;
use Illuminate\Validation\ValidationException;

class DocumentService
{
    public const ISMPT_TYPES = [
        Document::APPLICATION => 'KZ_APPLICATION_REPORT',
        Document::INTRODUCTION => 'KZ_LP_INTRODUCE_GOODS',
    ];

    private const SOURCE_STATUSES = [
        Document::APPLICATION => [Code::RECEIVED, Code::PRINTED],
        Document::INTRODUCTION => [Code::APPLIED],
    ];

    private const TARGET_STATUS = [
        Document::APPLICATION => Code::APPLIED,
        Document::INTRODUCTION => Code::INTRODUCED,
    ];

    private const MAX_POLLS = 60;

    public function __construct(private readonly IsmptSession $ismpt) {}

    public function prepare(User $user, string $type, array $selector): Document
    {
        $org = $user->organization;
        $codes = $this->selectCodes($org->id, $type, $selector);
        if ($codes->isEmpty()) {
            $hint = $type === Document::INTRODUCTION ? 'сначала передайте отчёт о нанесении' : 'нет полученных или напечатанных кодов';
            throw ValidationException::withMessages(['code_ids' => "Нет подходящих кодов: {$hint}"]);
        }

        $fileId = (string) Str::uuid();
        $body = [
            'fileId' => $fileId,
            'version' => '1.0',
            'sendingDateTime' => now()->utc()->format('Y-m-d\TH:i:s\Z'),
            'document' => [
                'mainInfo' => [
                    'participantInfo' => [
                        'participantName' => $org->name,
                        'participantIdentificationCode' => $org->bin,
                    ],
                    'originalDocInfo' => [
                        'docName' => Document::TYPE_LABELS[$type],
                        'docNumber' => strtoupper(substr($fileId, 0, 8)),
                        'docDate' => now()->format('d.m.Y'),
                    ],
                    'productionType' => 'LOCAL',
                ],
                'productList' => $codes->map(fn (Code $c) => ['iCode' => $c->code])->values()->all(),
            ],
        ];

        return DB::transaction(function () use ($user, $org, $type, $body, $codes) {
            $document = Document::create([
                'organization_id' => $org->id,
                'user_id' => $user->id,
                'type' => $type,
                'status' => Document::DRAFT,
                'body' => json_encode($body, JSON_UNESCAPED_UNICODE | JSON_UNESCAPED_SLASHES),
                'codes_count' => $codes->count(),
            ]);
            $document->codes()->attach($codes->pluck('id'));

            return $document;
        });
    }

    public function submit(Document $document, string $signature): Document
    {
        if ($document->status !== Document::DRAFT) {
            throw ValidationException::withMessages(['signature' => 'Документ уже отправлен']);
        }
        $org = $document->organization;
        $id = $this->ismpt->call($org, fn (TrueApiClient $c, string $t) => $c->createDocument($t, self::ISMPT_TYPES[$document->type], $document->body, $signature));
        $document->update(['status' => Document::SENT, 'ismpt_document_id' => $id, 'sent_at' => now()]);
        Operation::log($org->id, 'document.sent', Document::TYPE_LABELS[$document->type].": отправлен в ИС МПТ ({$document->codes_count} кодов)", subject: $document);
        ProcessDocument::dispatch($document->id)->delay(now()->addSecond());

        return $document;
    }

    public function poll(Document $document): bool
    {
        if (! in_array($document->status, [Document::SENT, Document::PROCESSING], true)) {
            return false;
        }
        $org = $document->organization;
        try {
            $info = $this->ismpt->call($org, fn (TrueApiClient $c, string $t) => $c->documentInfo($t, $document->ismpt_document_id));
        } catch (IsmptException $e) {
            if ($e->isUnauthorized() || $e->status === 503) {
                return false;
            }
            $document->update(['status' => Document::FAILED, 'errors' => [['code' => '', 'message' => $e->getMessage()]]]);

            return false;
        }
        $document->increment('attempts');
        $status = $info['status'] ?? 'IN_PROGRESS';

        if ($status === 'CHECKED_OK') {
            $this->accept($document);

            return false;
        }
        if (in_array($status, ['CHECKED_NOT_OK', 'PROCESSING_ERROR'], true)) {
            $errors = collect($info['cisErrors'] ?? [])->map(fn ($e) => ['code' => (string) ($e['cis'] ?? ''), 'message' => (string) ($e['errorMessage'] ?? '')])->all();
            if ($errors === []) {
                $errors = collect($info['errors'] ?? [])->map(fn ($e) => ['code' => '', 'message' => (string) $e])->all();
            }
            $document->update(['status' => Document::REJECTED, 'errors' => $errors, 'processed_at' => now()]);
            Operation::log($org->id, 'document.rejected', Document::TYPE_LABELS[$document->type].': отклонён ИС МПТ. '.($info['errorMessage'] ?? ''), 'error', $document, userId: null);

            return false;
        }
        $document->update(['status' => Document::PROCESSING]);

        return $document->attempts < self::MAX_POLLS;
    }

    private function accept(Document $document): void
    {
        $target = self::TARGET_STATUS[$document->type];
        $column = $target === Code::APPLIED ? 'applied_at' : 'introduced_at';
        DB::transaction(function () use ($document, $target, $column) {
            $now = now();
            Code::whereIn('id', $document->codes()->pluck('codes.id'))->update([
                'status' => $target,
                $column => $now,
                'updated_at' => $now,
            ]);
            $document->update(['status' => Document::ACCEPTED, 'errors' => [], 'processed_at' => $now]);
        });
        $what = $document->type === Document::APPLICATION ? 'нанесены' : 'введены в оборот';
        Operation::log($document->organization_id, 'document.accepted', Document::TYPE_LABELS[$document->type].": принят, {$document->codes_count} кодов {$what}", subject: $document, userId: null);
    }

    /**
     * @return Collection<int, Code>
     */
    private function selectCodes(int $orgId, string $type, array $selector): Collection
    {
        $query = Code::where('organization_id', $orgId)
            ->whereIn('status', self::SOURCE_STATUSES[$type])
            ->whereDoesntHave('documents', fn ($q) => $q->where('type', $type)->whereIn('status', [Document::DRAFT, Document::SENT, Document::PROCESSING])->where('documents.created_at', '>', now()->subHour()));
        if (! empty($selector['code_ids'])) {
            $query->whereIn('id', $selector['code_ids']);
        } elseif (! empty($selector['order_id'])) {
            $query->where('code_order_id', $selector['order_id']);
        } elseif (! empty($selector['product_id'])) {
            $query->where('product_id', $selector['product_id']);
        } else {
            throw ValidationException::withMessages(['code_ids' => 'Выберите коды, товар или заказ']);
        }

        return $query->orderBy('id')->limit(10000)->get();
    }
}
