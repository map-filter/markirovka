<?php

$mode = env('ISMPT_MODE', 'emulator');
$host = rtrim($mode === 'production' ? env('ISMPT_PRODUCTION_URL', 'https://elk.prod.markirovka.ismet.kz') : env('ISMPT_EMULATOR_URL', 'http://markirovka-emulator:8080'), '/');

return [
    'mode' => $mode,
    'host' => $host,
    'base_url' => $host.'/api/v3/true-api',
    'product_group' => env('ISMPT_PRODUCT_GROUP', 'lp'),
    'annul_days' => (int) env('ISMPT_ANNUL_DAYS', 30),
    'token_ttl_hours' => 10,
    'timeout' => (int) env('ISMPT_TIMEOUT', 20),
    'emulator_console_url' => env('EMULATOR_CONSOLE_URL', 'http://127.0.0.1:8303/emu/'),
    'sign_mode' => env('SIGN_MODE', 'emulate'),
    'remind_days' => (int) env('ISMPT_REMIND_DAYS', 7),
    'agent_max_wait' => 25,
    'seed_file' => env('SEED_FILE', base_path('../seed/seed.json')),
    'agent_token' => env('AGENT_TOKEN'),
];
