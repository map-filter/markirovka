<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\BelongsTo;

class Operation extends Model
{
    public const UPDATED_AT = null;

    protected $fillable = ['organization_id', 'user_id', 'type', 'level', 'message', 'subject_type', 'subject_id', 'payload'];

    protected function casts(): array
    {
        return ['payload' => 'array'];
    }

    public function user(): BelongsTo
    {
        return $this->belongsTo(User::class);
    }

    public static function log(int $organizationId, string $type, string $message, string $level = 'info', ?Model $subject = null, array $payload = [], ?int $userId = null): self
    {
        return static::create([
            'organization_id' => $organizationId,
            'user_id' => $userId ?? auth()->id(),
            'type' => $type,
            'level' => $level,
            'message' => $message,
            'subject_type' => $subject ? class_basename($subject) : null,
            'subject_id' => $subject?->getKey(),
            'payload' => $payload ?: null,
        ]);
    }
}
