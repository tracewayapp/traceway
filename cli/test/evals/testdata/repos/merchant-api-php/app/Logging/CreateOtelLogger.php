<?php

namespace App\Logging;

use Monolog\Logger;
use OpenTelemetry\API\Globals;
use OpenTelemetry\Contrib\Logs\Monolog\Handler;

class CreateOtelLogger
{
    public function __invoke(array $config): Logger
    {
        return new Logger($config['name'], [new Handler(Globals::loggerProvider(), $config['level'])]);
    }
}
