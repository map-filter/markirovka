<?php

namespace App\Console\Commands;

use App\Models\Organization;
use Illuminate\Console\Command;

class SeedIfEmpty extends Command
{
    protected $signature = 'app:seed-if-empty';

    protected $description = 'Заполняет базу начальными данными, если организаций ещё нет';

    public function handle(): int
    {
        if (Organization::exists()) {
            $this->line('Данные уже есть');

            return self::SUCCESS;
        }

        return $this->call('db:seed', ['--force' => true]);
    }
}
