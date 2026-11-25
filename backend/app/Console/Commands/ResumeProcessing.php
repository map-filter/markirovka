<?php

namespace App\Console\Commands;

use App\Jobs\ProcessCodeOrder;
use App\Jobs\ProcessDocument;
use App\Models\CodeOrder;
use App\Models\Document;
use Illuminate\Console\Command;

class ResumeProcessing extends Command
{
    protected $signature = 'ismpt:resume';

    protected $description = 'Повторно ставит в очередь зависшие заказы кодов и документы';

    public function handle(): int
    {
        $orders = CodeOrder::whereIn('status', CodeOrder::ACTIVE)->where('updated_at', '<', now()->subMinute())->pluck('id');
        $orders->each(fn (int $id) => ProcessCodeOrder::dispatch($id));
        $documents = Document::whereIn('status', [Document::SENT, Document::PROCESSING])->where('updated_at', '<', now()->subMinute())->pluck('id');
        $documents->each(fn (int $id) => ProcessDocument::dispatch($id));
        $this->line("Заказов: {$orders->count()}, документов: {$documents->count()}");

        return self::SUCCESS;
    }
}
