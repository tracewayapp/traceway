<?php

use App\Logging\CreateOtelLogger;
use Monolog\Handler\StreamHandler;

return [
    'default' => env('LOG_CHANNEL', 'stack'),

    'channels' => [
        'stack' => [
            'driver' => 'stack',
            'channels' => ['otel', 'stderr'],
            'ignore_exceptions' => false,
        ],

        'otel' => [
            'driver' => 'custom',
            'via' => CreateOtelLogger::class,
            'name' => 'merchant-api',
            'level' => env('LOG_LEVEL', 'info'),
        ],

        'stderr' => [
            'driver' => 'monolog',
            'handler' => StreamHandler::class,
            'with' => ['stream' => 'php://stderr'],
            'level' => env('LOG_LEVEL', 'info'),
        ],
    ],
];
