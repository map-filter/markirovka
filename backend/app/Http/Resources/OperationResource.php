<?php

namespace App\Http\Resources;

use Illuminate\Http\Request;
use Illuminate\Http\Resources\Json\JsonResource;

/** @mixin \App\Models\Operation */
class OperationResource extends JsonResource
{
    public function toArray(Request $request): array
    {
        return [
            'id' => $this->id,
            'created_at' => $this->created_at,
            'user' => $this->user?->name,
            'type' => $this->type,
            'level' => $this->level,
            'message' => $this->message,
        ];
    }
}
