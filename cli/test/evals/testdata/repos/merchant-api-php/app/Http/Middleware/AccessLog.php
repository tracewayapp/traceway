<?php

namespace App\Http\Middleware;

use Closure;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\Log;
use Symfony\Component\HttpFoundation\Response;

class AccessLog
{
    // One line per request ("GET /api/stores/12/orders 200 14ms"), inside the request's trace.
    public function handle(Request $request, Closure $next): Response
    {
        $start = hrtime(true);
        $response = $next($request);
        $ms = intdiv(hrtime(true) - $start, 1_000_000);
        $route = $request->route();

        Log::info(sprintf('%s %s %d %dms', $request->method(), $request->getPathInfo(), $response->getStatusCode(), $ms), [
            'http.request.method' => $request->method(),
            'url.path' => $request->getPathInfo(),
            'http.route' => $route ? '/'.ltrim($route->uri(), '/') : '',
            'http.response.status_code' => $response->getStatusCode(),
            'duration_ms' => $ms,
        ]);

        return $response;
    }
}
