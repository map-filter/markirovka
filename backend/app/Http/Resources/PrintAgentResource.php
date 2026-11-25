<?php

namespace App\Http\Resources;

use Illuminate\Http\Request;
use Illuminate\Http\Resources\Json\JsonResource;

/** @mixin \App\Models\PrintAgent */
class PrintAgentResource extends JsonResource
{
    public function toArray(Request $request): array
    {
        return [
            'id' => $this->id,
            'name' => $this->name,
            'hostname' => $this->hostname,
            'printer' => $this->printer,
            'version' => $this->version,
            'last_seen_at' => $this->last_seen_at,
            'online' => $this->isOnline(),
        ];
    }
}
