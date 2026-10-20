# merchant-api

The merchant dashboard API: stores, orders, products and reports. PHP 8.4,
Laravel 12 on MySQL 8, served by PHP-FPM behind nginx. Telemetry goes to
Traceway over OTLP through the OpenTelemetry PHP extension and
`open-telemetry/opentelemetry-auto-laravel` (`OTEL_PHP_AUTOLOAD_ENABLED=true`,
`OTEL_SERVICE_NAME=merchant-api`).

```
composer install
php artisan migrate
php artisan serve
```
