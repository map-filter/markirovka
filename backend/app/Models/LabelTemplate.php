<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\BelongsTo;

class LabelTemplate extends Model
{
    public const SIZES = [
        '58x40' => [58, 40],
        '43x25' => [43, 25],
    ];

    public const FIELDS = ['datamatrix', 'gtin', 'name', 'size', 'composition', 'manufacturer', 'serial'];

    protected $fillable = ['organization_id', 'name', 'size', 'fields', 'font_scale', 'is_default'];

    protected function casts(): array
    {
        return [
            'fields' => 'array',
            'font_scale' => 'float',
            'is_default' => 'boolean',
        ];
    }

    public function organization(): BelongsTo
    {
        return $this->belongsTo(Organization::class);
    }

    public function widthMm(): int
    {
        return self::SIZES[$this->size][0];
    }

    public function heightMm(): int
    {
        return self::SIZES[$this->size][1];
    }

    public function shows(string $field): bool
    {
        return (bool) ($this->fields[$field] ?? false);
    }
}
