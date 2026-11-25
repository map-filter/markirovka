<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\HasMany;

class Organization extends Model
{
    protected $fillable = ['name', 'bin', 'address', 'phone'];

    protected $hidden = ['integration_token_hash', 'ismpt_token'];

    protected function casts(): array
    {
        return [
            'ismpt_token' => 'encrypted',
            'ismpt_token_expires_at' => 'datetime',
        ];
    }

    public function users(): HasMany
    {
        return $this->hasMany(User::class);
    }

    public function products(): HasMany
    {
        return $this->hasMany(Product::class);
    }

    public function codeOrders(): HasMany
    {
        return $this->hasMany(CodeOrder::class);
    }

    public function codes(): HasMany
    {
        return $this->hasMany(Code::class);
    }

    public function labelTemplates(): HasMany
    {
        return $this->hasMany(LabelTemplate::class);
    }

    public function printAgents(): HasMany
    {
        return $this->hasMany(PrintAgent::class);
    }

    public function printJobs(): HasMany
    {
        return $this->hasMany(PrintJob::class);
    }

    public function documents(): HasMany
    {
        return $this->hasMany(Document::class);
    }

    public function operations(): HasMany
    {
        return $this->hasMany(Operation::class);
    }

    public function hasIsmptSession(): bool
    {
        return $this->ismpt_token !== null
            && $this->ismpt_token_expires_at !== null
            && $this->ismpt_token_expires_at->isFuture();
    }
}
