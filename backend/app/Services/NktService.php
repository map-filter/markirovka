<?php

namespace App\Services;

use App\Ismpt\IsmptSession;
use App\Ismpt\TrueApiClient;
use App\Models\Operation;
use App\Models\Product;

class NktService
{
    public function __construct(private readonly IsmptSession $ismpt) {}

    public function check(Product $product): ?array
    {
        $card = $this->ismpt->call($product->organization, fn (TrueApiClient $c, string $t) => $c->productInfo($t, $product->gtin));
        $product->update([
            'nkt_status' => $card ? Product::NKT_FOUND : Product::NKT_NOT_FOUND,
            'nkt_checked_at' => now(),
            'nkt_payload' => $card,
        ]);
        Operation::log(
            $product->organization_id,
            'nkt.checked',
            $card ? "Карточка {$product->gtin} найдена в НКТ" : "Карточки {$product->gtin} нет в НКТ",
            $card ? 'info' : 'warning',
            $product,
        );

        return $card;
    }

    public function register(Product $product): void
    {
        try {
            $this->ismpt->client()->registerCard([
                'gtin' => $product->gtin,
                'name' => $product->name,
                'model' => (string) $product->model,
                'size' => (string) $product->size,
                'color' => (string) $product->color,
                'composition' => (string) $product->composition,
                'tnved' => (string) $product->tnved,
                'category' => (string) $product->category,
                'brand' => (string) $product->brand,
                'country' => (string) $product->country,
                'participantBin' => $product->organization->bin,
            ]);
        } catch (\App\Ismpt\IsmptException $e) {
            if ($e->status !== 409) {
                throw $e;
            }
        }
        $product->update(['nkt_status' => Product::NKT_FOUND, 'nkt_checked_at' => now()]);
        Operation::log($product->organization_id, 'nkt.registered', "Карточка {$product->gtin} зарегистрирована в НКТ эмулятора", subject: $product);
    }

    public function syncAll(\App\Models\Organization $org): int
    {
        $gtins = array_flip($this->ismpt->call($org, fn (TrueApiClient $c, string $t) => $c->gtins($t)));
        $found = 0;
        foreach ($org->products as $product) {
            $exists = isset($gtins[$product->gtin]);
            $found += $exists ? 1 : 0;
            $product->update([
                'nkt_status' => $exists ? Product::NKT_FOUND : Product::NKT_NOT_FOUND,
                'nkt_checked_at' => now(),
            ]);
        }

        return $found;
    }
}
