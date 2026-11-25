<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\BelongsTo;
use Illuminate\Database\Eloquent\Relations\BelongsToMany;

class Document extends Model
{
    public const APPLICATION = 'application';
    public const INTRODUCTION = 'introduction';

    public const DRAFT = 'draft';
    public const SENT = 'sent';
    public const PROCESSING = 'processing';
    public const ACCEPTED = 'accepted';
    public const REJECTED = 'rejected';
    public const FAILED = 'failed';

    public const TYPE_LABELS = [
        self::APPLICATION => 'Отчёт о нанесении',
        self::INTRODUCTION => 'Ввод в оборот',
    ];

    protected $fillable = [
        'organization_id', 'user_id', 'type', 'status', 'body', 'ismpt_document_id', 'errors', 'codes_count',
        'attempts', 'sent_at', 'processed_at',
    ];

    protected function casts(): array
    {
        return [
            'errors' => 'array',
            'sent_at' => 'datetime',
            'processed_at' => 'datetime',
        ];
    }

    public function organization(): BelongsTo
    {
        return $this->belongsTo(Organization::class);
    }

    public function codes(): BelongsToMany
    {
        return $this->belongsToMany(Code::class);
    }
}
