<?php

namespace Database\Seeders;

use App\Ismpt\EmulatedSignature;
use App\Ismpt\IsmptSession;
use App\Ismpt\TrueApiClient;
use App\Models\Code;
use App\Models\CodeOrder;
use App\Models\Document;
use App\Models\LabelTemplate;
use App\Models\Operation;
use App\Models\Organization;
use App\Models\PrintAgent;
use App\Models\PrintJob;
use App\Models\Product;
use App\Models\User;
use App\Services\CodeOrderService;
use App\Services\DocumentService;
use App\Services\PrintService;
use Illuminate\Database\Seeder;
use Throwable;

class DatabaseSeeder extends Seeder
{
    public function run(): void
    {
        config(['queue.default' => 'null']);
        $seed = json_decode((string) file_get_contents(config('ismpt.seed_file')), true);

        $org = Organization::create([
            'name' => $seed['participant']['name'],
            'bin' => $seed['participant']['bin'],
            'address' => $seed['participant']['address'],
            'phone' => '+7 7182 55-01-23',
        ]);
        $admin = User::create([
            'organization_id' => $org->id,
            'name' => 'Айгерим Нурланова',
            'email' => 'admin@kendir.test',
            'role' => 'admin',
            'password' => 'portal2026',
        ]);
        User::create([
            'organization_id' => $org->id,
            'name' => 'Ерлан Сапаров',
            'email' => 'ceh@kendir.test',
            'role' => 'operator',
            'password' => 'portal2026',
        ]);

        foreach ($seed['products'] as $item) {
            Product::create(array_intersect_key($item, array_flip(Product::ATTRIBUTES)) + ['organization_id' => $org->id]);
        }
        Product::create([
            'organization_id' => $org->id,
            'gtin' => '04870712009990',
            'name' => 'Поло мужское с вышивкой, белый, р. L',
            'model' => 'PL-M-30',
            'size' => 'L',
            'color' => 'белый',
            'composition' => 'хлопок 95%, эластан 5%',
            'tnved' => '6105100000',
            'category' => 'футболки',
            'brand' => 'Кендір',
            'country' => 'Казахстан',
        ]);

        LabelTemplate::create([
            'organization_id' => $org->id,
            'name' => 'Ярлык одежды 58x40',
            'size' => '58x40',
            'fields' => array_fill_keys(LabelTemplate::FIELDS, true),
            'font_scale' => 1,
            'is_default' => true,
        ]);
        LabelTemplate::create([
            'organization_id' => $org->id,
            'name' => 'Бельё 43x25',
            'size' => '43x25',
            'fields' => ['datamatrix' => true, 'gtin' => true, 'name' => true, 'size' => true, 'composition' => false, 'manufacturer' => true, 'serial' => true],
            'font_scale' => 1,
        ]);

        if ($token = config('ismpt.agent_token')) {
            PrintAgent::create([
                'organization_id' => $org->id,
                'name' => 'Цех, Zebra ZD421',
                'token_hash' => PrintAgent::hashToken($token),
            ]);
        }

        Operation::log($org->id, 'org.created', 'Организация подключена к сервису', userId: $admin->id);

        if (config('ismpt.mode') === 'emulator') {
            try {
                $this->history($org, $admin);
            } catch (Throwable $e) {
                $this->command?->warn('Эмулятор ИС МПТ недоступен, история заказов не создана: '.$e->getMessage());
            }
        }
    }

    private function history(Organization $org, User $admin): void
    {
        $session = app(IsmptSession::class);
        $client = $session->client();
        $key = $client->authKey();
        $session->connect($org, $key['uuid'], EmulatedSignature::sign($key['data'], $org->bin));
        $org->refresh();

        $known = array_flip($client->gtins($org->ismpt_token));
        Product::where('organization_id', $org->id)->get()->each(fn (Product $p) => $p->update([
            'nkt_status' => isset($known[$p->gtin]) ? Product::NKT_FOUND : Product::NKT_NOT_FOUND,
            'nkt_checked_at' => now(),
        ]));

        $products = Product::where('organization_id', $org->id)->get()->keyBy('model');
        $orders = app(CodeOrderService::class);
        $tee = $this->order($orders, $admin, Product::where('organization_id', $org->id)->where('gtin', '04870712001017')->first(), 40);
        $hoodie = $this->order($orders, $admin, $products['HD-U-10'], 30);
        $suit = $this->order($orders, $admin, $products['SP-01'], 20);
        $boxers = $this->order($orders, $admin, $products['BL-M-21'], 24);

        $template = LabelTemplate::where('organization_id', $org->id)->where('is_default', true)->first();
        $print = app(PrintService::class);
        $print->create($admin, ['template_id' => $template->id, 'format' => 'pdf', 'target' => 'download', 'order_id' => $tee->id, 'quantity' => 30]);
        $print->create($admin, ['template_id' => $template->id, 'format' => 'zpl', 'target' => 'download', 'order_id' => $suit->id]);

        $teeCodes = Code::where('code_order_id', $tee->id)->where('status', Code::PRINTED)->orderBy('id')->pluck('id');
        $this->document($org, $admin, Document::APPLICATION, ['code_ids' => $teeCodes->take(20)->all()]);
        $this->document($org, $admin, Document::INTRODUCTION, ['code_ids' => $teeCodes->take(10)->all()]);
        $this->document($org, $admin, Document::APPLICATION, ['order_id' => $suit->id]);
        $this->document($org, $admin, Document::INTRODUCTION, ['order_id' => $suit->id]);

        Code::where('code_order_id', $hoodie->id)->update([
            'emitted_at' => now()->subDays(25),
            'expires_at' => now()->subDays(25)->addDays((int) config('ismpt.annul_days')),
        ]);
        $hoodie->update(['expires_at' => now()->subDays(25)->addDays((int) config('ismpt.annul_days')), 'created_at' => now()->subDays(25), 'ready_at' => now()->subDays(25)]);

        CodeOrder::create([
            'organization_id' => $org->id,
            'user_id' => $admin->id,
            'product_id' => $products['SP-02']->id,
            'number' => CodeOrder::nextNumber($org->id),
            'quantity' => 15000,
            'status' => CodeOrder::FAILED,
            'error' => 'Параметр quantity требует значение от 1 до 10000',
            'created_at' => now()->subDays(2),
            'updated_at' => now()->subDays(2),
        ]);

        PrintJob::where('organization_id', $org->id)->update(['created_at' => now()->subDays(3)]);
        $org->forceFill(['ismpt_token' => null, 'ismpt_token_expires_at' => null])->save();
    }

    private function order(CodeOrderService $service, User $user, Product $product, int $quantity): CodeOrder
    {
        $order = $service->create($user, $product, $quantity);
        for ($i = 0; $i < 30 && $service->process($order->refresh()); $i++) {
            usleep(1_000_000);
        }

        return $order->refresh();
    }

    private function document(Organization $org, User $user, string $type, array $selector): void
    {
        $service = app(DocumentService::class);
        $document = $service->prepare($user, $type, $selector);
        $service->submit($document, EmulatedSignature::sign($document->body, $org->bin));
        for ($i = 0; $i < 20 && $service->poll($document->refresh()); $i++) {
            usleep(1_000_000);
        }
    }
}
