<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\BelongsTo;
use Illuminate\Database\Eloquent\Relations\BelongsToMany;

class PrintJob extends Model
{
    public const READY = 'ready';
    public const QUEUED = 'queued';
    public const SENT = 'sent';
    public const PRINTED = 'printed';
    public const FAILED = 'failed';

    protected $fillable = [
        'organization_id', 'user_id', 'label_template_id', 'print_agent_id', 'format', 'target', 'status',
        'codes_count', 'file_path', 'error', 'sent_at', 'printed_at',
    ];

    protected function casts(): array
    {
        return [
            'sent_at' => 'datetime',
            'printed_at' => 'datetime',
        ];
    }

    public function template(): BelongsTo
    {
        return $this->belongsTo(LabelTemplate::class, 'label_template_id');
    }

    public function agent(): BelongsTo
    {
        return $this->belongsTo(PrintAgent::class, 'print_agent_id');
    }

    public function codes(): BelongsToMany
    {
        return $this->belongsToMany(Code::class);
    }
}
