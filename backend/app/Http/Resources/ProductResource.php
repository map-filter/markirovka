<?php

namespace App\Http\Resources;

use App\Models\Code;
use Illuminate\Http\Request;
use Illuminate\Http\Resources\Json\JsonResource;

/** @mixin \App\Models\Product */
class ProductResource extends JsonResource
{
    public function toArray(Request $request): array
    {
        return [
            'id' => $this->id,
            'gtin' => $this->gtin,
            'name' => $this->name,
            'model' => $this->model,
            'size' => $this->size,
            'color' => $this->color,
            'composition' => $this->composition,
            'tnved' => $this->tnved,
            'category' => $this->category,
            'brand' => $this->brand,
            'country' => $this->country,
            'nkt_status' => $this->nkt_status,
            'nkt_checked_at' => $this->nkt_checked_at,
            'codes' => $this->resource->getAttribute('code_counts') ?? Code::emptyCounts(),
            'created_at' => $this->created_at,
        ];
    }
}
