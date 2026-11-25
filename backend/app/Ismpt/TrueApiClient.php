<?php

namespace App\Ismpt;

use Illuminate\Http\Client\ConnectionException;
use Illuminate\Http\Client\PendingRequest;
use Illuminate\Http\Client\Response;
use Illuminate\Support\Facades\Http;
use ZipArchive;

class TrueApiClient
{
    public function __construct(
        private readonly string $baseUrl,
        private readonly string $host,
        private readonly string $productGroup,
        private readonly int $timeout = 20,
    ) {}

    public static function fromConfig(): self
    {
        return new self(config('ismpt.base_url'), config('ismpt.host'), config('ismpt.product_group'), config('ismpt.timeout'));
    }

    /**
     * @return array{uuid: string, data: string}
     */
    public function authKey(): array
    {
        $json = $this->send(fn () => $this->http()->get('/auth/key'));

        return ['uuid' => (string) $json['uuid'], 'data' => (string) $json['data']];
    }

    public function signIn(string $uuid, string $signature): string
    {
        $json = $this->send(fn () => $this->http()->post('/auth/simpleSignIn', ['uuid' => $uuid, 'data' => $signature]));

        return (string) $json['token'];
    }

    /**
     * @return list<string>
     */
    public function gtins(string $token, int $limit = 10000): array
    {
        $json = $this->send(fn () => $this->http($token)->get('/product/gtin', ['pg' => $this->productGroup, 'limit' => $limit, 'page' => 0]));

        return $json['gtins'] ?? [];
    }

    public function productInfo(string $token, string $gtin): ?array
    {
        try {
            return $this->send(fn () => $this->http($token)->get('/products/info', ['cis' => $gtin]));
        } catch (IsmptException $e) {
            if ($e->status === 404) {
                return null;
            }
            throw $e;
        }
    }

    public function registerCard(array $card): array
    {
        return $this->send(fn () => Http::baseUrl($this->host)->timeout($this->timeout)->acceptJson()->post('/emu/api/nkt/cards', $card));
    }

    public function orderCodes(string $token, string $gtin, int $quantity): string
    {
        $json = $this->send(fn () => $this->http($token)->post('/cises/my', [
            'gtin' => $gtin,
            'quantity' => $quantity,
            'packageType' => 'UNIT',
        ]));

        return (string) $json['uuid'];
    }

    public function orderStatus(string $token, string $uuid): string
    {
        $json = $this->send(fn () => $this->http($token)->get("/cises/orders/{$uuid}/status"));

        return str_replace(' ', '_', (string) $json['status']);
    }

    /**
     * @return list<array<string, mixed>>
     */
    public function orderResult(string $token, string $uuid): array
    {
        $response = $this->raw(fn () => $this->http($token)->accept('*/*')->get("/cises/orders/{$uuid}/result"));
        $body = $response->body();
        if (str_starts_with($body, "PK\x03\x04")) {
            $body = $this->unzipFirst($body);
        }
        $json = json_decode($body, true);
        if (! is_array($json)) {
            throw new IsmptException('ИС МПТ вернула результат заказа в неизвестном формате');
        }

        return $json;
    }

    /**
     * @param  list<string>  $codes
     * @return list<array<string, mixed>>
     */
    public function cisesInfo(string $token, array $codes): array
    {
        try {
            return $this->send(fn () => $this->http($token)->post('/cises/info', $codes));
        } catch (IsmptException $e) {
            if ($e->status === 404) {
                return [];
            }
            throw $e;
        }
    }

    public function createDocument(string $token, string $type, string $document, string $signature): string
    {
        $json = $this->send(fn () => $this->http($token)->post('/documents/create', [
            'type' => $type,
            'document' => base64_encode($document),
            'signature' => $signature,
        ]));

        return (string) $json['documentId'];
    }

    public function documentInfo(string $token, string $documentId): array
    {
        return $this->send(fn () => $this->http($token)->get("/documents/{$documentId}/info"));
    }

    private function http(?string $token = null): PendingRequest
    {
        $request = Http::baseUrl($this->baseUrl)->timeout($this->timeout)->acceptJson();

        return $token ? $request->withToken($token) : $request;
    }

    /**
     * @param  callable(): Response  $call
     */
    private function send(callable $call): array
    {
        $json = $this->raw($call)->json();

        return is_array($json) ? $json : [];
    }

    /**
     * @param  callable(): Response  $call
     */
    private function raw(callable $call): Response
    {
        try {
            $response = $call();
        } catch (ConnectionException $e) {
            throw new IsmptException('ИС МПТ недоступна: '.$e->getMessage(), 503);
        }
        if ($response->successful()) {
            return $response;
        }
        throw new IsmptException($this->errorMessage($response), $response->status());
    }

    private function errorMessage(Response $response): string
    {
        if ($response->status() === 401) {
            return 'Сессия ИС МПТ истекла или подпись не принята: подключитесь заново';
        }
        $json = $response->json();
        $message = is_array($json) ? ($json['error_message'] ?? $json['errorMessage'] ?? $json['description'] ?? null) : null;

        return $message ? (string) $message : "ИС МПТ ответила кодом {$response->status()}";
    }

    private function unzipFirst(string $zipBytes): string
    {
        $path = tempnam(sys_get_temp_dir(), 'ismpt');
        file_put_contents($path, $zipBytes);
        $zip = new ZipArchive;
        $opened = $zip->open($path) === true;
        try {
            if (! $opened || $zip->numFiles < 1) {
                throw new IsmptException('Не удалось открыть ZIP с кодами');
            }

            return (string) $zip->getFromIndex(0);
        } finally {
            if ($opened) {
                $zip->close();
            }
            @unlink($path);
        }
    }
}
