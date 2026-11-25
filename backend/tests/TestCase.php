<?php

namespace Tests;

use App\Models\Code;
use App\Models\CodeOrder;
use App\Models\LabelTemplate;
use App\Models\Organization;
use App\Models\Product;
use App\Models\User;
use App\Support\Gs1;
use Illuminate\Foundation\Testing\TestCase as BaseTestCase;
use Illuminate\Support\Str;

abstract class TestCase extends BaseTestCase
{
    protected function organization(bool $connected = true): Organization
    {
        $org = Organization::create(['name' => 'ТОО «Тест Тігін»', 'bin' => (string) random_int(100000000000, 999999999999), 'address' => 'г. Павлодар']);
        if ($connected) {
            $org->forceFill(['ismpt_token' => 'test-token', 'ismpt_token_expires_at' => now()->addHours(9)])->save();
        }

        return $org;
    }

    protected function userFor(Organization $org): User
    {
        return User::create([
            'organization_id' => $org->id,
            'name' => 'Тестовый пользователь',
            'email' => Str::random(8).'@kendir.test',
            'role' => 'admin',
            'password' => 'portal2026',
        ]);
    }

    protected function product(Organization $org, string $gtin = '04870712001017', string $nkt = Product::NKT_FOUND): Product
    {
        return Product::create([
            'organization_id' => $org->id,
            'gtin' => $gtin,
            'name' => 'Футболка мужская базовая, белый, р. M',
            'size' => 'M',
            'composition' => 'хлопок 100%',
            'nkt_status' => $nkt,
        ]);
    }

    protected function template(Organization $org, string $size = '58x40'): LabelTemplate
    {
        return LabelTemplate::create([
            'organization_id' => $org->id,
            'name' => 'Ярлык',
            'size' => $size,
            'fields' => array_fill_keys(LabelTemplate::FIELDS, true),
            'is_default' => true,
        ]);
    }

    protected function sampleCode(string $gtin = '04870712001017', ?string $serial = null): string
    {
        $serial ??= Str::random(13);

        return '01'.$gtin.'21'.$serial.Gs1::GS.'91EE10'.Gs1::GS.'92'.str_repeat('Ab0+', 11);
    }

    /**
     * @return list<Code>
     */
    protected function codes(Product $product, int $count, string $status = Code::RECEIVED): array
    {
        $order = CodeOrder::create([
            'organization_id' => $product->organization_id,
            'product_id' => $product->id,
            'number' => CodeOrder::nextNumber($product->organization_id),
            'quantity' => $count,
            'status' => CodeOrder::READY,
            'ismpt_order_id' => (string) Str::uuid(),
        ]);
        $codes = [];
        for ($i = 0; $i < $count; $i++) {
            $code = $this->sampleCode($product->gtin);
            $parsed = Gs1::parse($code);
            $codes[] = Code::create([
                'organization_id' => $product->organization_id,
                'product_id' => $product->id,
                'code_order_id' => $order->id,
                'code' => $code,
                'cis' => $parsed['cis'],
                'gtin' => $parsed['gtin'],
                'serial' => $parsed['serial'],
                'status' => $status,
                'emitted_at' => now(),
                'expires_at' => now()->addDays(30),
            ]);
        }

        return $codes;
    }
}
