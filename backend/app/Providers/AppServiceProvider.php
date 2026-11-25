<?php

namespace App\Providers;

use App\Ismpt\TrueApiClient;
use Illuminate\Support\ServiceProvider;

class AppServiceProvider extends ServiceProvider
{
    public function register(): void
    {
        $this->app->bind(TrueApiClient::class, fn () => TrueApiClient::fromConfig());
    }
}
