<?php

namespace App\Jobs;

use App\Models\CodeOrder;
use App\Services\CodeOrderService;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Foundation\Queue\Queueable;

class ProcessCodeOrder implements ShouldQueue
{
    use Queueable;

    public int $tries = 1;

    public function __construct(public readonly int $orderId) {}

    public function handle(CodeOrderService $service): void
    {
        $order = CodeOrder::find($this->orderId);
        if ($order && $service->process($order)) {
            self::dispatch($this->orderId)->delay(now()->addSeconds(2));
        }
    }
}
