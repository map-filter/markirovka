<?php

namespace App\Http\Requests;

use App\Rules\Gtin;
use App\Support\Gs1;
use Illuminate\Foundation\Http\FormRequest;
use Illuminate\Validation\Rule;

class ProductRequest extends FormRequest
{
    public function rules(): array
    {
        $id = $this->route('id');

        return [
            'gtin' => [
                'required', 'string', new Gtin,
                Rule::unique('products', 'gtin')
                    ->where('organization_id', $this->user()->organization_id)
                    ->ignore($id),
            ],
            'name' => ['required', 'string', 'max:500'],
            'model' => ['nullable', 'string', 'max:100'],
            'size' => ['nullable', 'string', 'max:50'],
            'color' => ['nullable', 'string', 'max:100'],
            'composition' => ['nullable', 'string', 'max:255'],
            'tnved' => ['nullable', 'regex:/^\d{10}$/'],
            'category' => ['nullable', 'string', 'max:100'],
            'brand' => ['nullable', 'string', 'max:100'],
            'country' => ['nullable', 'string', 'max:100'],
        ];
    }

    public function messages(): array
    {
        return [
            'gtin.unique' => 'Товар с таким GTIN уже есть',
            'tnved.regex' => 'Код ТН ВЭД: 10 цифр',
        ];
    }

    protected function prepareForValidation(): void
    {
        if (is_string($this->input('gtin'))) {
            $gtin = preg_replace('/\D/', '', $this->input('gtin'));
            $this->merge(['gtin' => strlen($gtin) >= 12 && strlen($gtin) <= 14 ? Gs1::normalizeGtin($gtin) : $gtin]);
        }
        if (is_string($this->input('tnved'))) {
            $this->merge(['tnved' => preg_replace('/\D/', '', $this->input('tnved')) ?: null]);
        }
    }

    public function productData(): array
    {
        return $this->safe()->only(\App\Models\Product::ATTRIBUTES);
    }
}
