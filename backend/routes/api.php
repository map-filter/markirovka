<?php

use App\Http\Controllers\Api\AgentApiController;
use App\Http\Controllers\Api\AuthController;
use App\Http\Controllers\Api\CodeController;
use App\Http\Controllers\Api\CodeOrderController;
use App\Http\Controllers\Api\DashboardController;
use App\Http\Controllers\Api\DocumentController;
use App\Http\Controllers\Api\IntegrationController;
use App\Http\Controllers\Api\IsmptController;
use App\Http\Controllers\Api\LabelTemplateController;
use App\Http\Controllers\Api\OperationController;
use App\Http\Controllers\Api\PrintJobController;
use App\Http\Controllers\Api\ProductController;
use App\Http\Controllers\Api\SettingsController;
use Illuminate\Support\Facades\Route;

Route::post('auth/login', [AuthController::class, 'login'])->middleware('throttle:10,1');
Route::get('config', [AuthController::class, 'config']);

Route::prefix('agent')->group(function () {
    Route::get('jobs/next', [AgentApiController::class, 'next']);
    Route::post('jobs/{id}/ack', [AgentApiController::class, 'ack']);
    Route::post('heartbeat', [AgentApiController::class, 'heartbeat']);
});

Route::prefix('integration/v1')->middleware('throttle:120,1')->group(function () {
    Route::get('codes', [IntegrationController::class, 'codes']);
    Route::post('codes/statuses', [IntegrationController::class, 'statuses']);
    Route::get('orders', [IntegrationController::class, 'orders']);
});

Route::get('print-jobs/{id}/download', [PrintJobController::class, 'download'])->middleware('auth.query-token', 'auth:sanctum');

Route::middleware('auth:sanctum')->group(function () {
    Route::post('auth/logout', [AuthController::class, 'logout']);
    Route::get('me', [AuthController::class, 'me']);
    Route::get('dashboard', DashboardController::class);

    Route::get('ismpt/session', [IsmptController::class, 'session']);
    Route::post('ismpt/auth/key', [IsmptController::class, 'key']);
    Route::post('ismpt/auth/sign-in', [IsmptController::class, 'signIn']);
    Route::post('sign/emulate', [IsmptController::class, 'emulateSign']);

    Route::get('products/categories', [ProductController::class, 'categories']);
    Route::get('products/import/template', [ProductController::class, 'template']);
    Route::post('products/import', [ProductController::class, 'import']);
    Route::post('products/{id}/nkt-check', [ProductController::class, 'nktCheck']);
    Route::post('products/{id}/nkt-register', [ProductController::class, 'nktRegister']);
    Route::get('products', [ProductController::class, 'index']);
    Route::post('products', [ProductController::class, 'store']);
    Route::get('products/{id}', [ProductController::class, 'show']);
    Route::put('products/{id}', [ProductController::class, 'update']);
    Route::delete('products/{id}', [ProductController::class, 'destroy']);

    Route::get('code-orders', [CodeOrderController::class, 'index']);
    Route::post('code-orders', [CodeOrderController::class, 'store']);
    Route::get('code-orders/{id}', [CodeOrderController::class, 'show']);
    Route::post('code-orders/{id}/refresh', [CodeOrderController::class, 'refresh']);

    Route::get('codes/summary', [CodeController::class, 'summary']);
    Route::get('codes', [CodeController::class, 'index']);

    Route::get('label-templates', [LabelTemplateController::class, 'index']);
    Route::post('label-templates', [LabelTemplateController::class, 'store']);
    Route::put('label-templates/{id}', [LabelTemplateController::class, 'update']);
    Route::delete('label-templates/{id}', [LabelTemplateController::class, 'destroy']);
    Route::get('label-templates/{id}/preview', [LabelTemplateController::class, 'preview']);

    Route::get('print-jobs', [PrintJobController::class, 'index']);
    Route::post('print-jobs', [PrintJobController::class, 'store']);
    Route::get('print-jobs/{id}', [PrintJobController::class, 'show']);

    Route::get('documents', [DocumentController::class, 'index']);
    Route::post('documents/prepare', [DocumentController::class, 'prepare']);
    Route::get('documents/{id}', [DocumentController::class, 'show']);
    Route::post('documents/{id}/submit', [DocumentController::class, 'submit']);

    Route::get('operations', [OperationController::class, 'index']);

    Route::get('settings', [SettingsController::class, 'show']);
    Route::put('settings', [SettingsController::class, 'update']);
    Route::post('settings/integration-token', [SettingsController::class, 'integrationToken']);
    Route::post('agents', [SettingsController::class, 'storeAgent']);
    Route::delete('agents/{id}', [SettingsController::class, 'destroyAgent']);
});
