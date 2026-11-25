<?php

namespace App\Labels;

use App\Support\Gs1;

final class LabelItem
{
    public function __construct(
        public readonly string $code,
        public readonly string $name,
        public readonly ?string $size,
        public readonly ?string $composition,
        public readonly string $gtin,
        public readonly string $manufacturer,
    ) {}

    public function serialLine(): string
    {
        $parsed = Gs1::parse($this->code);

        return '(01)'.$parsed['gtin'].'(21)'.$parsed['serial'];
    }
}
