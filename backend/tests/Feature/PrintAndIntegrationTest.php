<?php

namespace Tests\Feature;

use App\Models\Code;
use App\Models\Operation;
use App\Models\PrintAgent;
use App\Models\PrintJob;
use App\Services\CodeLifecycle;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Tests\TestCase;

class PrintAndIntegrationTest extends TestCase
{
    use RefreshDatabase;

    public function test_download_print_job_marks_codes_printed(): void
    {
        $org = $this->organization();
        $product = $this->product($org);
        $this->codes($product, 4);
        $template = $this->template($org);
        $user = $this->userFor($org);
        $this->actingAs($user);

        $job = $this->postJson('/api/print-jobs', ['template_id' => $template->id, 'format' => 'pdf', 'target' => 'download', 'product_id' => $product->id, 'quantity' => 3])
            ->assertCreated()->assertJsonPath('data.codes_count', 3)->json('data');

        $this->assertSame(3, Code::where('status', Code::PRINTED)->count());
        $response = $this->get($job['download_url']);
        $response->assertOk()->assertHeader('Content-Type', 'application/pdf');
        $this->assertStringStartsWith('%PDF-', $response->streamedContent());

        $this->postJson('/api/print-jobs', ['template_id' => $template->id, 'format' => 'zpl', 'target' => 'download', 'product_id' => $product->id])
            ->assertCreated()->assertJsonPath('data.codes_count', 1);
        $this->postJson('/api/print-jobs', ['template_id' => $template->id, 'format' => 'zpl', 'target' => 'download', 'product_id' => $product->id])
            ->assertStatus(422);
    }

    public function test_agent_receives_job_and_acknowledges(): void
    {
        $org = $this->organization();
        $product = $this->product($org);
        $this->codes($product, 2);
        $template = $this->template($org, '43x25');
        $agent = PrintAgent::create(['organization_id' => $org->id, 'name' => 'Цех', 'token_hash' => PrintAgent::hashToken('agent-secret')]);
        $this->actingAs($this->userFor($org));
        $job = $this->postJson('/api/print-jobs', ['template_id' => $template->id, 'format' => 'zpl', 'target' => 'agent', 'product_id' => $product->id])
            ->assertCreated()->assertJsonPath('data.status', 'queued')->json('data');
        $this->assertSame(0, Code::where('status', Code::PRINTED)->count());

        $this->getJson('/api/agent/jobs/next?wait=0', ['Authorization' => 'Bearer wrong'])->assertStatus(401);
        $next = $this->getJson('/api/agent/jobs/next?wait=0', ['Authorization' => 'Bearer agent-secret'])
            ->assertOk()->assertJsonPath('id', (string) $job['id'])->assertJsonPath('format', 'zpl')->json();
        $zpl = base64_decode($next['payload_base64']);
        $this->assertStringStartsWith('^XA', $zpl);
        $this->assertStringContainsString('~1', $zpl);
        $this->getJson('/api/agent/jobs/next?wait=0', ['Authorization' => 'Bearer agent-secret'])->assertNoContent();

        $this->postJson("/api/agent/jobs/{$next['id']}/ack", ['status' => 'printed'], ['Authorization' => 'Bearer agent-secret'])->assertNoContent();
        $this->postJson("/api/agent/jobs/{$next['id']}/ack", ['status' => 'printed'], ['Authorization' => 'Bearer agent-secret'])->assertNoContent();
        $this->assertSame(PrintJob::PRINTED, PrintJob::find($job['id'])->status);
        $this->assertSame(2, Code::where('status', Code::PRINTED)->count());

        $this->postJson('/api/agent/heartbeat', ['version' => '1.0.0', 'hostname' => 'ceh-pc', 'printer' => 'tcp://192.168.1.50:9100'], ['Authorization' => 'Bearer agent-secret'])->assertNoContent();
        $this->assertTrue($agent->fresh()->isOnline());
    }

    public function test_integration_export_for_1c(): void
    {
        $org = $this->organization();
        $product = $this->product($org);
        $codes = $this->codes($product, 2, Code::INTRODUCED);
        $this->actingAs($this->userFor($org));
        $token = $this->postJson('/api/settings/integration-token')->assertOk()->json('token');
        auth()->forgetGuards();

        $this->getJson('/api/integration/v1/codes', ['Authorization' => 'Bearer nope'])->assertStatus(401);
        $json = $this->getJson('/api/integration/v1/codes?order='.urlencode($codes[0]->codeOrder->number), ['Authorization' => "Bearer {$token}"])
            ->assertOk()->assertJsonCount(2, 'codes')->assertJsonPath('codes.0.status_label', 'В обороте')->json();
        $this->assertSame($codes[0]->code, base64_decode($json['codes'][0]['code_base64']));

        $csv = $this->get('/api/integration/v1/codes?format=csv', ['Authorization' => "Bearer {$token}"])->assertOk()->streamedContent();
        $this->assertStringContainsString('code_base64;code_hr', $csv);
        $this->assertSame(3, substr_count(trim($csv), "\n") + 1);

        $this->postJson('/api/integration/v1/codes/statuses', ['codes' => [\App\Support\Gs1::humanReadable($codes[1]->code), 'garbage']], ['Authorization' => "Bearer {$token}"])
            ->assertOk()->assertJsonPath('codes.0.status', 'introduced')->assertJsonPath('not_found.0', 'garbage');
    }

    public function test_expiring_codes_reminder_and_annulment(): void
    {
        $org = $this->organization(connected: false);
        $product = $this->product($org);
        $soon = $this->codes($product, 2);
        $late = $this->codes($product, 1, Code::PRINTED);
        Code::whereIn('id', collect($soon)->pluck('id'))->update(['expires_at' => now()->addDays(3)]);
        Code::where('id', $late[0]->id)->update(['expires_at' => now()->subHour()]);
        $this->actingAs($this->userFor($org));

        $this->getJson('/api/dashboard')->assertOk()
            ->assertJsonPath('ismpt.connected', false)
            ->assertJsonPath('todo.0.kind', 'connect_ismpt')
            ->assertJsonPath('expiring.0.count', 2);

        $lifecycle = app(CodeLifecycle::class);
        $this->assertSame(1, $lifecycle->remindExpiring($org));
        $this->assertSame(0, $lifecycle->remindExpiring($org));
        $this->assertSame(1, $lifecycle->annulExpired($org));
        $this->assertSame(Code::ANNULLED, $late[0]->fresh()->status);
        $this->assertTrue(Operation::where('type', 'codes.expiring')->exists());
    }
}
