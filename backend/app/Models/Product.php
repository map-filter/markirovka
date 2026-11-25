<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Factories\HasFactory;
use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\BelongsTo;
use Illuminate\Database\Eloquent\Relations\HasMany;

class Product extends Model
{
    use HasFactory;

    public const NKT_UNCHECKED = 'unchecked';
    public const NKT_FOUND = 'found';
    public const NKT_NOT_FOUND = 'not_found';

    public const ATTRIBUTES = ['gtin', 'name', 'model', 'size', 'color', 'composition', 'tnved', 'category', 'brand', 'country'];

    protected $fillable = [
        'organization_id', 'gtin', 'name', 'model', 'size', 'color', 'composition', 'tnved', 'category', 'brand', 'country',
        'nkt_status', 'nkt_checked_at', 'nkt_payload',
    ];

    protected function casts(): array
    {
        return [
            'nkt_checked_at' => 'datetime',
            'nkt_payload' => 'array',
        ];
    }

    public function organization(): BelongsTo
    {
        return $this->belongsTo(Organization::class);
    }

    public function codes(): HasMany
    {
        return $this->hasMany(Code::class);
    }

    public function codeOrders(): HasMany
    {
        return $this->hasMany(CodeOrder::class);
    }
}
