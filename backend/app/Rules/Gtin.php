<?php

namespace App\Rules;

use App\Support\Gs1;
use Closure;
use Illuminate\Contracts\Validation\ValidationRule;

class Gtin implements ValidationRule
{
    public function validate(string $attribute, mixed $value, Closure $fail): void
    {
        if (! is_string($value) || ! preg_match('/^\d{14}$/', $value)) {
            $fail('GTIN: 13 или 14 цифр');

            return;
        }
        if (! Gs1::isValidGtin($value)) {
            $fail('Неверная контрольная цифра GTIN');
        }
    }
}
