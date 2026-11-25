<?php

namespace Tests\Feature;

use App\Ismpt\EmulatedSignature;
use App\Models\Code;
use App\Models\CodeOrder;
use App\Models\Document;
use App\Models\Product;
use App\Support\Gs1;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Illuminate\Support\Facades\Http;
use Tests\TestCase;

class IsmptFlowTest extends TestCase
{
    use RefreshDatabase;

    private const API = 'ismpt.test/api/v3/true-api';

    public function test_connect_with_emulated_signature(): void
    {
        $org = $this->organization(connected: false);
        $this->actingAs($this->userFor($org));
        Http::fake([
            self::API.'/auth/key' => Http::response(['uuid' => 'u-1', 'data' => 'ABCDEF']),
            self::API.'/auth/simpleSignIn' => Http::response(['token' => 'jwt-token']),
            self::API.'/product/gtin*' => Http::response(['gtins' => [], 'total' => 0]),
        ]);

        $key = $this->postJson('/api/ismpt/auth/key')->assertOk()->json();
        $signature = $this->postJson('/api/sign/emulate', ['data_base64' => base64_encode($key['data'])])->assertOk()->json('signature');
        $this->assertSame(EmulatedSignature::sign('ABCDEF', $org->bin), $signature);

        $this->postJson('/api/ismpt/auth/sign-in', ['uuid' => $key['uuid'], 'signature' => $signature])
            ->assertOk()->assertJsonPath('connected', true);
        $this->assertSame('jwt-token', $org->fresh()->ismpt_token);
        Http::assertSent(fn ($r) => str_ends_with($r->url(), '/auth/simpleSignIn') && $r['data'] === $signature && $r['uuid'] === 'u-1');
    }

    public function test_order_requires_session_and_nkt_card(): void
    {
        $org = $this->organization(connected: false);
        $product = $this->product($org);
        $this->actingAs($this->userFor($org));
        $this->postJson('/api/code-orders', ['product_id' => $product->id, 'quantity' => 5])->assertStatus(409);

        $org->forceFill(['ismpt_token' => 't', 'ismpt_token_expires_at' => now()->addHour()])->save();
        $product->update(['nkt_status' => Product::NKT_NOT_FOUND]);
        auth()->user()->unsetRelation('organization');
        $this->postJson('/api/code-orders', ['product_id' => $product->id, 'quantity' => 5])
            ->assertStatus(409)->assertJsonPath('message', 'Товара нет в НКТ: сначала проверьте или зарегистрируйте карточку');
    }

    public function test_order_codes_and_receive_them_with_gs_preserved(): void
    {
        $org = $this->organization();
        $product = $this->product($org);
        $this->actingAs($this->userFor($org));
        $codes = [$this->sampleCode(serial: 'AAAAAAAAAAAA1'), $this->sampleCode(serial: 'AAAAAAAAAAAA2')];
        Http::fake([
            self::API.'/cises/my' => Http::response(['uuid' => 'order-1']),
            self::API.'/cises/orders/order-1/status' => Http::response(['status' => 'SUCCESS']),
            self::API.'/cises/orders/order-1/result' => Http::response($this->zip(array_map(fn ($c) => [
                'cis' => Gs1::humanReadable($c), 'printCode' => $c, 'gtin' => $product->gtin, 'status' => 'EMITTED', 'emissionDate' => '2026-11-25T10:00:00.000Z',
            ], $codes)), 200, ['Content-Type' => 'application/zip']),
        ]);

        $id = $this->postJson('/api/code-orders', ['product_id' => $product->id, 'quantity' => 2])
            ->assertCreated()->json('data.id');

        $order = CodeOrder::find($id);
        $this->assertSame(CodeOrder::READY, $order->status);
        $this->assertSame('З-000001', $order->number);
        $this->assertSame('2026-12-25', $order->expires_at->toDateString());
        $stored = Code::where('code_order_id', $id)->orderBy('id')->pluck('code')->all();
        $this->assertSame($codes, $stored);
        $this->assertSame(2, substr_count($stored[0], "\x1D"));
        Http::assertSent(fn ($r) => str_ends_with($r->url(), '/cises/my') && $r['quantity'] === 2 && $r['gtin'] === $product->gtin);

        $this->getJson("/api/code-orders/{$id}")->assertJsonPath('data.codes.received', 2);
        $this->getJson('/api/codes?search=AAAAAAAAAAAA1')->assertJsonCount(1, 'data')
            ->assertJsonPath('data.0.code_hr', Gs1::humanReadable($codes[0]));
    }

    public function test_session_is_dropped_on_unauthorized(): void
    {
        $org = $this->organization();
        $product = $this->product($org);
        $this->actingAs($this->userFor($org));
        Http::fake([self::API.'/cises/my' => Http::response('<InvalidTokenException/>', 401)]);

        $this->postJson('/api/code-orders', ['product_id' => $product->id, 'quantity' => 2])->assertCreated();

        $this->assertNull($org->fresh()->ismpt_token);
        $this->assertSame(CodeOrder::NEW, CodeOrder::first()->status);
    }

    public function test_application_then_introduction(): void
    {
        $org = $this->organization();
        $product = $this->product($org);
        $codes = $this->codes($product, 3, Code::PRINTED);
        $this->actingAs($this->userFor($org));
        Http::fake([
            self::API.'/documents/create' => Http::sequence()->push(['documentId' => 'doc-1'])->push(['documentId' => 'doc-2']),
            self::API.'/documents/doc-1/info' => Http::response(['status' => 'CHECKED_OK', 'errors' => [], 'cisErrors' => []]),
            self::API.'/documents/doc-2/info' => Http::response([
                'status' => 'CHECKED_NOT_OK',
                'errorMessage' => 'Документ не прошёл проверку',
                'cisErrors' => [['cis' => $codes[0]->cis, 'errorCode' => 'GS_LOST', 'errorMessage' => 'потерян символ GS']],
            ]),
        ]);

        $this->postJson('/api/documents/prepare', ['type' => 'introduction', 'order_id' => $codes[0]->code_order_id])
            ->assertStatus(422);

        $doc = $this->postJson('/api/documents/prepare', ['type' => 'application', 'order_id' => $codes[0]->code_order_id])
            ->assertCreated()->assertJsonPath('data.codes_count', 3)->json('data');
        $body = json_decode(base64_decode($doc['body_base64']), true);
        $this->assertSame($org->bin, $body['document']['mainInfo']['participantInfo']['participantIdentificationCode']);
        $this->assertSame($codes[0]->code, $body['document']['productList'][0]['iCode']);

        $signature = EmulatedSignature::sign(base64_decode($doc['body_base64']), $org->bin);
        $this->postJson("/api/documents/{$doc['id']}/submit", ['signature' => $signature])->assertOk();
        $this->assertSame(Document::ACCEPTED, Document::find($doc['id'])->status);
        $this->assertSame(3, Code::where('status', Code::APPLIED)->count());
        Http::assertSent(fn ($r) => str_ends_with($r->url(), '/documents/create') && $r['type'] === 'KZ_APPLICATION_REPORT' && $r['signature'] === $signature);

        $doc2 = $this->postJson('/api/documents/prepare', ['type' => 'introduction', 'code_ids' => [$codes[0]->id]])->assertCreated()->json('data');
        $this->postJson("/api/documents/{$doc2['id']}/submit", ['signature' => 'x'])->assertOk();
        $this->getJson("/api/documents/{$doc2['id']}")
            ->assertJsonPath('data.status', 'rejected')
            ->assertJsonPath('data.errors.0.message', 'потерян символ GS');
        $this->assertSame(Code::APPLIED, $codes[0]->fresh()->status);
    }

    private function zip(array $items): string
    {
        $path = tempnam(sys_get_temp_dir(), 'zip');
        $zip = new \ZipArchive;
        $zip->open($path, \ZipArchive::OVERWRITE);
        $zip->addFromString('order-1.json', json_encode($items));
        $zip->close();
        $bytes = (string) file_get_contents($path);
        unlink($path);

        return $bytes;
    }
}
