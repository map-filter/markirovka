<?php

namespace App\Http\Resources;

use Illuminate\Http\Request;
use Illuminate\Http\Resources\Json\JsonResource;

/** @mixin \App\Models\PrintJob */
class PrintJobResource extends JsonResource
{
    public function toArray(Request $request): array
    {
        return [
            'id' => $this->id,
            'template' => new LabelTemplateResource($this->whenLoaded('template')),
            'format' => $this->format,
            'target' => $this->target,
            'status' => $this->status,
            'codes_count' => $this->codes_count,
            'agent' => new PrintAgentResource($this->whenLoaded('agent')),
            'error' => $this->error,
            'download_url' => '/api/print-jobs/'.$this->id.'/download',
            'created_at' => $this->created_at,
        ];
    }
}
