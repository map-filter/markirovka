<?php

namespace App\Http\Resources;

use App\Support\Gs1;
use Illuminate\Http\Request;
use Illuminate\Http\Resources\Json\JsonResource;

/** @mixin \App\Models\Code */
class CodeResource extends JsonResource
{
    public function toArray(Request $request): array
    {
        return [
            'id' => $this->id,
            'code' => $this->code,
            'code_hr' => Gs1::humanReadable($this->code),
            'gtin' => $this->gtin,
            'serial' => $this->serial,
            'status' => $this->status,
            'product' => new ProductShortResource($this->whenLoaded('product')),
            'order_id' => $this->code_order_id,
            'order_number' => $this->whenLoaded('codeOrder', fn () => $this->codeOrder->number),
            'emitted_at' => $this->emitted_at,
            'expires_at' => $this->expires_at,
            'printed_at' => $this->printed_at,
            'applied_at' => $this->applied_at,
            'introduced_at' => $this->introduced_at,
        ];
    }
}
