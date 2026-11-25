<?php

use App\Http\Middleware\QueryTokenToHeader;
use App\Ismpt\IsmptException;
use Illuminate\Auth\AuthenticationException;
use Illuminate\Database\Eloquent\ModelNotFoundException;
use Illuminate\Foundation\Application;
use Illuminate\Foundation\Configuration\Exceptions;
use Illuminate\Foundation\Configuration\Middleware;
use Illuminate\Http\Request;
use Symfony\Component\HttpKernel\Exception\NotFoundHttpException;

return Application::configure(basePath: dirname(__DIR__))
    ->withRouting(
        web: __DIR__.'/../routes/web.php',
        api: __DIR__.'/../routes/api.php',
        commands: __DIR__.'/../routes/console.php',
        health: '/up',
    )
    ->withMiddleware(function (Middleware $middleware): void {
        $middleware->alias(['auth.query-token' => QueryTokenToHeader::class]);
    })
    ->withExceptions(function (Exceptions $exceptions): void {
        $exceptions->shouldRenderJsonWhen(fn (Request $request) => $request->is('api/*') || $request->expectsJson());
        $exceptions->render(fn (IsmptException $e) => response()->json([
            'message' => $e->getMessage(),
            'ismpt_status' => $e->status,
        ], match (true) {
            $e->isUnauthorized() => 409,
            $e->status >= 500 || $e->status === 0 => 502,
            default => 409,
        }));
        $exceptions->render(fn (AuthenticationException $e, Request $request) => response()->json(['message' => 'Требуется вход'], 401));
        $exceptions->render(function (NotFoundHttpException $e, Request $request) {
            if ($e->getPrevious() instanceof ModelNotFoundException) {
                return response()->json(['message' => 'Не найдено'], 404);
            }
        });
    })->create();
