<?php

use App\Http\Middleware\AccessLog;
use Illuminate\Foundation\Application;
use Illuminate\Foundation\Configuration\Exceptions;
use Illuminate\Foundation\Configuration\Middleware;

return Application::configure(basePath: dirname(__DIR__))
    ->withRouting(
        api: __DIR__.'/../routes/api.php',
    )
    ->withMiddleware(function (Middleware $middleware): void {
        $middleware->append(AccessLog::class);
    })
    ->withExceptions(function (Exceptions $exceptions): void {})
    ->create();
