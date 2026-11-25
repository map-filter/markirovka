<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\BelongsTo;
use Illuminate\Database\Eloquent\Relations\HasMany;

class CodeOrder extends Model
{
    public const NEW = 'new';
    public const SENT = 'sent';
    public const PROCESSING = 'processing';
    public const READY = 'ready';
    public const FAILED = 'failed';

    public const ACTIVE = [self::NEW, self::SENT, self::PROCESSING];

    protected $fillable = [
        'organization_id', 'user_id', 'product_id', 'number', 'quantity', 'status', 'ismpt_order_id', 'error',
        'attempts', 'sent_at', 'ready_at', 'expires_at',
    ];

    protected function casts(): array
    {
        return [
            'sent_at' => 'datetime',
            'ready_at' => 'datetime',
            'expires_at' => 'datetime',
        ];
    }

    public function organization(): BelongsTo
    {
        return $this->belongsTo(Organization::class);
    }

    public function product(): BelongsTo
    {
        return $this->belongsTo(Product::class);
    }

    public function user(): BelongsTo
    {
        return $this->belongsTo(User::class);
    }

    public function codes(): HasMany
    {
        return $this->hasMany(Code::class);
    }

    public static function nextNumber(int $organizationId): string
    {
        $count = static::where('organization_id', $organizationId)->count();

        return sprintf('З-%06d', $count + 1);
    }
}
