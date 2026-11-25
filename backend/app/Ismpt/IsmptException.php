<?php

namespace App\Ismpt;

use RuntimeException;

class IsmptException extends RuntimeException
{
    public function __construct(string $message, public readonly int $status = 0)
    {
        parent::__construct($message, $status);
    }

    public function isUnauthorized(): bool
    {
        return $this->status === 401;
    }
}
