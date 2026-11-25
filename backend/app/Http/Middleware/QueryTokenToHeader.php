<?php

namespace App\Http\Middleware;

use Closure;
use Illuminate\Http\Request;
use Symfony\Component\HttpFoundation\Response;

class QueryTokenToHeader
{
    public function handle(Request $request, Closure $next): Response
    {
        if (! $request->bearerToken() && is_string($token = $request->query('token'))) {
            $request->headers->set('Authorization', 'Bearer '.$token);
        }

        return $next($request);
    }
}
