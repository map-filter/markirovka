<?php

namespace App\Ismpt;

use App\Models\Operation;
use App\Models\Organization;

class IsmptSession
{
    public function __construct(private readonly TrueApiClient $client) {}

    public function client(): TrueApiClient
    {
        return $this->client;
    }

    public function token(Organization $org): string
    {
        if (! $org->hasIsmptSession()) {
            throw new IsmptException('Нет подключения к ИС МПТ: подпишите вход ЭЦП на главной странице', 401);
        }

        return $org->ismpt_token;
    }

    public function connect(Organization $org, string $uuid, string $signature): void
    {
        $token = $this->client->signIn($uuid, $signature);
        $org->forceFill([
            'ismpt_token' => $token,
            'ismpt_token_expires_at' => now()->addHours(config('ismpt.token_ttl_hours'))->subMinutes(5),
        ])->save();
        Operation::log($org->id, 'ismpt.connected', 'Подключение к ИС МПТ по ЭЦП, токен действует 10 часов');
    }

    public function forget(Organization $org): void
    {
        if ($org->ismpt_token === null) {
            return;
        }
        $org->forceFill(['ismpt_token' => null, 'ismpt_token_expires_at' => null])->save();
        Operation::log($org->id, 'ismpt.disconnected', 'Сессия ИС МПТ закончилась, нужен повторный вход по ЭЦП', 'warning', userId: null);
    }

    /**
     * @template T
     *
     * @param  callable(TrueApiClient, string): T  $call
     * @return T
     */
    public function call(Organization $org, callable $call): mixed
    {
        try {
            return $call($this->client, $this->token($org));
        } catch (IsmptException $e) {
            if ($e->isUnauthorized() && $org->ismpt_token !== null) {
                $this->forget($org);
            }
            throw $e;
        }
    }

    public function status(Organization $org): array
    {
        return [
            'connected' => $org->hasIsmptSession(),
            'expires_at' => $org->hasIsmptSession() ? $org->ismpt_token_expires_at : null,
            'mode' => config('ismpt.mode'),
        ];
    }
}
