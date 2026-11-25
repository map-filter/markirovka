<?php

namespace App\Http\Resources;

use App\Models\Document;
use Illuminate\Http\Request;
use Illuminate\Http\Resources\Json\JsonResource;

/** @mixin \App\Models\Document */
class DocumentResource extends JsonResource
{
    public function toArray(Request $request): array
    {
        return [
            'id' => $this->id,
            'type' => $this->type,
            'type_label' => Document::TYPE_LABELS[$this->type] ?? $this->type,
            'status' => $this->status,
            'codes_count' => $this->codes_count,
            'body_base64' => base64_encode($this->body),
            'ismpt_document_id' => $this->ismpt_document_id,
            'errors' => $this->errors ?? [],
            'created_at' => $this->created_at,
            'processed_at' => $this->processed_at,
        ];
    }
}
