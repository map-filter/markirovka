<?php

namespace Tests\Feature;

use App\Models\Product;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Illuminate\Http\UploadedFile;
use Illuminate\Support\Facades\Http;
use Tests\TestCase;

class AuthAndProductsTest extends TestCase
{
    use RefreshDatabase;

    public function test_login_returns_token_and_rejects_wrong_password(): void
    {
        $user = $this->userFor($this->organization());

        $this->postJson('/api/auth/login', ['email' => $user->email, 'password' => 'wrong'])->assertStatus(422);
        $token = $this->postJson('/api/auth/login', ['email' => $user->email, 'password' => 'portal2026'])
            ->assertOk()->json('token');

        $this->withToken($token)->getJson('/api/me')->assertOk()->assertJsonPath('organization.name', 'ТОО «Тест Тігін»');
    }

    public function test_api_requires_authentication(): void
    {
        $this->getJson('/api/products')->assertStatus(401);
    }

    public function test_product_crud_with_gtin_validation(): void
    {
        $user = $this->userFor($this->organization());
        $this->actingAs($user);

        $this->postJson('/api/products', ['gtin' => '04870712001018', 'name' => 'Футболка'])
            ->assertStatus(422)->assertJsonValidationErrors('gtin');

        $id = $this->postJson('/api/products', ['gtin' => '4870712001017', 'name' => 'Футболка', 'tnved' => '6109 10 000 0'])
            ->assertCreated()
            ->assertJsonPath('data.gtin', '04870712001017')
            ->assertJsonPath('data.tnved', '6109100000')
            ->assertJsonPath('data.nkt_status', 'unchecked')
            ->json('data.id');

        $this->postJson('/api/products', ['gtin' => '04870712001017', 'name' => 'Дубль'])->assertStatus(422);
        $this->putJson("/api/products/{$id}", ['gtin' => '04870712001017', 'name' => 'Футболка базовая'])->assertOk()->assertJsonPath('data.name', 'Футболка базовая');
        $this->getJson('/api/products?search=базовая')->assertOk()->assertJsonCount(1, 'data');
        $this->deleteJson("/api/products/{$id}")->assertNoContent();
    }

    public function test_products_are_isolated_between_organizations(): void
    {
        $other = $this->product($this->organization());
        $this->actingAs($this->userFor($this->organization()));

        $this->getJson("/api/products/{$other->id}")->assertNotFound();
        $this->getJson('/api/products')->assertJsonCount(0, 'data');
    }

    public function test_import_csv_creates_updates_and_reports_errors(): void
    {
        $org = $this->organization();
        $this->product($org);
        $this->actingAs($this->userFor($org));
        $csv = "\xEF\xBB\xBFGTIN;Наименование;Размер;Состав;ТН ВЭД\n"
            ."04870712001017;Футболка обновлённая;M;хлопок 100%;6109100000\n"
            ."04870712001024;Футболка новая;L;хлопок 100%;6109100000\n"
            ."04870712001025;Ошибка в GTIN;L;;\n";
        $file = UploadedFile::fake()->createWithContent('tovary.csv', $csv);

        $this->post('/api/products/import', ['file' => $file], ['Accept' => 'application/json'])
            ->assertOk()
            ->assertJsonPath('created', 1)
            ->assertJsonPath('updated', 1)
            ->assertJsonPath('errors.0.row', 4);
        $this->assertSame('Футболка обновлённая', Product::where('gtin', '04870712001017')->value('name'));
    }

    public function test_nkt_check_marks_product(): void
    {
        $org = $this->organization();
        $product = $this->product($org, nkt: Product::NKT_UNCHECKED);
        $this->actingAs($this->userFor($org));
        Http::fake(['ismpt.test/api/v3/true-api/products/info*' => Http::response(['id' => 1, 'gtin' => $product->gtin, 'name' => 'Футболка'])]);

        $this->postJson("/api/products/{$product->id}/nkt-check")->assertOk()->assertJsonPath('data.nkt_status', 'found');
        Http::assertSent(fn ($request) => $request->hasHeader('Authorization', 'Bearer test-token'));
    }
}
