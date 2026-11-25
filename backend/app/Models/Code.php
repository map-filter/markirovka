<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\BelongsTo;

class Code extends Model
{
    public const RECEIVED = 'received';
    public const PRINTED = 'printed';
    public const APPLIED = 'applied';
    public const INTRODUCED = 'introduced';
    public const ANNULLED = 'annulled';

    public const STATUSES = [self::RECEIVED, self::PRINTED, self::APPLIED, self::INTRODUCED, self::ANNULLED];

    public const LABELS = [
        self::RECEIVED => 'Получен',
        self::PRINTED => 'Напечатан',
        self::APPLIED => 'Нанесён',
        self::INTRODUCED => 'В обороте',
        self::ANNULLED => 'Аннулирован',
    ];

    protected $fillable = [
        'organization_id', 'product_id', 'code_order_id', 'code', 'cis', 'gtin', 'serial', 'status',
        'emitted_at', 'expires_at', 'printed_at', 'applied_at', 'introduced_at', 'annulled_at',
    ];

    protected function casts(): array
    {
        return [
            'emitted_at' => 'datetime',
            'expires_at' => 'datetime',
            'printed_at' => 'datetime',
            'applied_at' => 'datetime',
            'introduced_at' => 'datetime',
            'annulled_at' => 'datetime',
        ];
    }

    public function product(): BelongsTo
    {
        return $this->belongsTo(Product::class);
    }

    public function documents(): \Illuminate\Database\Eloquent\Relations\BelongsToMany
    {
        return $this->belongsToMany(Document::class);
    }

    public function codeOrder(): BelongsTo
    {
        return $this->belongsTo(CodeOrder::class);
    }

    /**
     * @param  \Illuminate\Support\Collection<int, Model>  $models
     */
    public static function attachCounts(\Illuminate\Support\Collection $models, string $column): void
    {
        if ($models->isEmpty()) {
            return;
        }
        $rows = static::query()
            ->whereIn($column, $models->pluck('id'))
            ->selectRaw("{$column} as owner_id, status, count(*) as total")
            ->groupBy($column, 'status')
            ->get();
        $map = [];
        foreach ($rows as $row) {
            $map[$row->owner_id][$row->status] = (int) $row->total;
        }
        foreach ($models as $model) {
            $model->setAttribute('code_counts', array_merge(self::emptyCounts(), $map[$model->id] ?? []));
        }
    }

    public static function emptyCounts(): array
    {
        return array_fill_keys(self::STATUSES, 0);
    }
}
