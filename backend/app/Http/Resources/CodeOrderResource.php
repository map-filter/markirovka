<?php

namespace App\Http\Resources;

use App\Models\Code;
use Illuminate\Http\Request;
use Illuminate\Http\Resources\Json\JsonResource;

/** @mixin \App\Models\CodeOrder */
class CodeOrderResource extends JsonResource
{
    public function toArray(Request $request): array
    {
        return [
            'id' => $this->id,
            'number' => $this->number,
            'product' => new ProductShortResource($this->whenLoaded('product')),
            'quantity' => $this->quantity,
            'status' => $this->status,
            'ismpt_order_id' => $this->ismpt_order_id,
            'error' => $this->error,
            'codes' => $this->resource->getAttribute('code_counts') ?? Code::emptyCounts(),
            'created_at' => $this->created_at,
            'ready_at' => $this->ready_at,
            'expires_at' => $this->expires_at,
        ];
    }
}
