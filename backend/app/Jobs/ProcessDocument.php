<?php

namespace App\Jobs;

use App\Models\Document;
use App\Services\DocumentService;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Foundation\Queue\Queueable;

class ProcessDocument implements ShouldQueue
{
    use Queueable;

    public int $tries = 1;

    public function __construct(public readonly int $documentId) {}

    public function handle(DocumentService $service): void
    {
        $document = Document::find($this->documentId);
        if ($document && $service->poll($document)) {
            self::dispatch($this->documentId)->delay(now()->addSeconds(2));
        }
    }
}
