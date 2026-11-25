<?php

namespace App\Http\Resources;

use App\Models\LabelTemplate;
use Illuminate\Http\Request;
use Illuminate\Http\Resources\Json\JsonResource;

/** @mixin \App\Models\LabelTemplate */
class LabelTemplateResource extends JsonResource
{
    public function toArray(Request $request): array
    {
        $fields = [];
        foreach (LabelTemplate::FIELDS as $field) {
            $fields[$field] = $this->shows($field);
        }

        return [
            'id' => $this->id,
            'name' => $this->name,
            'size' => $this->size,
            'width_mm' => $this->widthMm(),
            'height_mm' => $this->heightMm(),
            'fields' => $fields,
            'font_scale' => $this->font_scale,
            'is_default' => $this->is_default,
        ];
    }
}
