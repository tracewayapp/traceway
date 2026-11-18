# merchant-api

The merchant dashboard API: stores, orders, products and reports. PHP 8.4,
Laravel 12 on MySQL 8, served by PHP-FPM behind nginx. API tokens are Sanctum
personal access tokens. Telemetry goes to Traceway over OTLP through the
OpenTelemetry PHP extension and `open-telemetry/opentelemetry-auto-laravel`
(`OTEL_PHP_AUTOLOAD_ENABLED=true`, `OTEL_SERVICE_NAME=merchant-api`); logs go
through the `otel` Monolog channel (`OTEL_PHP_MONOLOG_ATTRIB_MODE=otel`).

```
composer install
php artisan migrate
php artisan serve
```
