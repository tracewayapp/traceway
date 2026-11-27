import type { Framework } from '$lib/state/projects.svelte';
import { isJsFramework } from '$lib/state/projects.svelte';
import { normalizeBaseUrl } from './connection-string';
import { SDK_VERSION, getStartCode, usesStartApi } from './sdk-start';

// Legacy Go and Node.js projects (LEGACY_GO_FRAMEWORKS, LEGACY_NODE_FRAMEWORKS) get the OpenTelemetry setup
// guide instead, so the retired Go and 1.x Node.js SDKs have no install command or snippet here.
export function getInstallCommand(framework: Framework): string {
	switch (framework) {
		case 'react':
			return `npm install @tracewayapp/react@${SDK_VERSION}`;
		case 'svelte':
			return `npm install @tracewayapp/svelte@${SDK_VERSION}`;
		case 'vuejs':
			return `npm install @tracewayapp/vue@${SDK_VERSION}`;
		case 'jquery':
			return `npm install @tracewayapp/jquery@${SDK_VERSION}`;
		case 'react-native':
			return 'npm install @tracewayapp/react-native';
		case 'hono':
			return '';
		case 'symfony':
			return 'composer require traceway/opentelemetry-symfony open-telemetry/exporter-otlp php-http/guzzle7-adapter';
		case 'laravel':
			return 'composer require keepsuit/laravel-opentelemetry open-telemetry/exporter-otlp php-http/guzzle7-adapter';
		case 'django':
			return 'pip install opentelemetry-distro opentelemetry-exporter-otlp opentelemetry-instrumentation-django && opentelemetry-bootstrap -a install';
		case 'cloudflare':
			return '';
		case 'opentelemetry':
			return '';
		case 'flutter':
			return `flutter pub add 'traceway:^${SDK_VERSION}'`;
		case 'android':
			return `implementation("com.tracewayapp:traceway:${SDK_VERSION}")`;
		case 'ios':
			return `.package(url: "https://github.com/tracewayapp/traceway-ios.git", from: "${SDK_VERSION}")`;
		default:
			return '';
	}
}

// Browser and mobile projects get the start API of the 2.1 SDKs (token + baseUrl, see sdk-start.ts). React
// Native is still on its 1.x SDK, which posts to the 1.x /api/report endpoint itself, so its snippet alone keeps
// a connection string.
export function getFrameworkCode(framework: Framework, token: string, backendUrl: string): string {
	if (usesStartApi(framework)) return getStartCode(framework, backendUrl);

	const base = normalizeBaseUrl(backendUrl) ?? backendUrl;
	const connectionString = `${token || 'YOUR_TOKEN'}@${base}/api/report`;

	switch (framework) {
		case 'react-native':
			return `import { TracewayProvider } from "@tracewayapp/react-native";

export default function App() {
  return (
    <TracewayProvider connectionString="${connectionString}">
      <RootNavigator />
    </TracewayProvider>
  );
}`;

		case 'symfony':
			return `<?php
// public/index.php

use App\\Kernel;

require_once dirname(__DIR__) . '/vendor/autoload.php';

\\OpenTelemetry\\SDK\\SdkAutoloader::autoload();

// Fixes for Symfony's OTel auto-instrumentation:
// 1. Corrects http.route from internal route name to URL path template
// 2. Cleans up sub-request scopes so 500 error spans are exported
\\OpenTelemetry\\Instrumentation\\hook(
    \\Symfony\\Component\\HttpKernel\\HttpKernel::class,
    'handle',
    post: static function (
        \\Symfony\\Component\\HttpKernel\\HttpKernel $kernel,
        array $params,
        mixed $returnValue,
        ?\\Throwable $exception
    ): void {
        $request = ($params[0] instanceof \\Symfony\\Component\\HttpFoundation\\Request) ? $params[0] : null;
        if (null === $request) return;

        $type = $params[1] ?? \\Symfony\\Component\\HttpKernel\\HttpKernelInterface::MAIN_REQUEST;

        if ($type === \\Symfony\\Component\\HttpKernel\\HttpKernelInterface::SUB_REQUEST) {
            $scope = \\OpenTelemetry\\Context\\Context::storage()->scope();
            if (null !== $scope) {
                $span = \\OpenTelemetry\\API\\Trace\\Span::fromContext($scope->context());
                $scope->detach();
                $span->end();
            }
            return;
        }

        $routeParams = $request->attributes->get('_route_params', []);
        $path = $request->getPathInfo();
        if (\\is_array($routeParams)) {
            foreach ($routeParams as $name => $value) {
                if (\\is_string($value) && '' !== $value) {
                    $path = str_replace($value, '{' . $name . '}', $path);
                }
            }
        }

        $request->attributes->set('_route', $path);
    }
);

$kernel = new Kernel($_SERVER['APP_ENV'] ?? 'dev', (bool) ($_SERVER['APP_DEBUG'] ?? true));
$request = \\Symfony\\Component\\HttpFoundation\\Request::createFromGlobals();
$response = $kernel->handle($request);
$response->send();
$kernel->terminate($request, $response);`;

		case 'laravel':
			return `<?php
// .env  - point the OTLP exporter at Traceway
//
// OTEL_SERVICE_NAME=my-laravel-app
// OTEL_TRACES_EXPORTER=otlp
// OTEL_METRICS_EXPORTER=otlp
// OTEL_LOGS_EXPORTER=otlp
// OTEL_EXPORTER_OTLP_PROTOCOL=http/json
// OTEL_EXPORTER_OTLP_ENDPOINT=${base}/api/otel
// OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer ${token || 'YOUR_TOKEN'}"
//
// Optional: send Laravel logs to Traceway via the auto-injected 'otlp' channel
// LOG_CHANNEL=otlp

// That's it - keepsuit/laravel-opentelemetry's service provider auto-registers
// TraceRequestMiddleware as a global middleware, so every HTTP request, DB query,
// queued job, Redis call, cache op, view render and outbound Http:: call is
// traced automatically. Open config/opentelemetry.php to tune which
// instrumentations are enabled.`;

		case 'django':
			return `# .env  - point the OTLP exporter at Traceway
#
# OTEL_SERVICE_NAME=my-django-app
# OTEL_TRACES_EXPORTER=otlp
# OTEL_METRICS_EXPORTER=otlp
# OTEL_LOGS_EXPORTER=otlp
# OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf
# OTEL_EXPORTER_OTLP_ENDPOINT=${base}/api/otel
# OTEL_EXPORTER_OTLP_HEADERS=Authorization=Bearer%20${token || 'YOUR_TOKEN'}
# OTEL_PYTHON_LOGGING_AUTO_INSTRUMENTATION_ENABLED=true

# Then launch Django through the OTel agent - no code changes needed:
#
#   opentelemetry-instrument python manage.py runserver
#   opentelemetry-instrument gunicorn myproject.wsgi:application
#
# DjangoInstrumentor auto-installs middleware at index 0 and traces every
# inbound request. opentelemetry-bootstrap also wired psycopg/redis/requests/
# celery/logging instrumentation, so DB queries, cache ops, outbound HTTP
# and queued tasks are traced automatically.`;

		case 'hono':
			return '';

		case 'cloudflare':
			return '';

		case 'opentelemetry':
			return '';

		default:
			return '';
	}
}

export function getTestingRouteCode(framework?: Framework): string {
	if (framework === 'symfony') {
		return `<?php
// src/Controller/TestController.php
namespace App\\Controller;

use Symfony\\Component\\HttpFoundation\\Response;
use Symfony\\Component\\Routing\\Attribute\\Route;

class TestController
{
    #[Route('/testing', name: 'testing')]
    public function index(): Response
    {
        throw new \\RuntimeException("Test error from Traceway integration");
    }
}`;
	}
	if (framework === 'laravel') {
		return `<?php
// routes/web.php
use Illuminate\\Support\\Facades\\Route;

Route::get('/testing', function () {
    throw new \\RuntimeException('Test error from Traceway integration');
});`;
	}
	if (framework === 'django') {
		return `# myapp/views.py
from django.http import HttpResponse


def testing(request):
    raise RuntimeError("Test error from Traceway integration")


# myproject/urls.py
from django.urls import path
from myapp import views

urlpatterns = [
    path("testing/", views.testing),
]`;
	}
	if (framework === 'flutter') {
		return `// Trigger a test error
throw StateError('Test error from Traceway integration');`;
	}
	if (framework === 'android') {
		return `// Trigger a test error
throw RuntimeException("Test error from Traceway integration")`;
	}
	if (framework === 'ios') {
		return `// Send a test error (a caught one works with the debugger attached)
struct TestError: Error {}
Traceway.captureException(TestError())
Traceway.flush(timeout: 5)`;
	}
	if (framework && isJsFramework(framework)) {
		return `// Trigger a test error
throw new Error("Test error from Traceway integration");`;
	}
	return '';
}

export function getTestingRouteCode2(framework?: Framework): string {
	if (framework === 'nextjs') return '';
	if (framework === 'symfony') {
		return '';
	}
	if (framework === 'laravel') {
		return '';
	}
	if (framework === 'django') {
		return '';
	}
	if (framework === 'flutter') {
		return `import 'package:traceway/traceway.dart';

Traceway.captureException(
  Exception('Test error'),
  StackTrace.current,
);`;
	}
	if (framework === 'android') {
		return `import com.tracewayapp.traceway.Traceway

try {
    riskyOperation()
} catch (e: Throwable) {
    Traceway.captureException(e)
}`;
	}
	if (framework === 'ios') {
		return `import Traceway

do {
    try riskyOperation()
} catch {
    Traceway.captureException(error)
}`;
	}
	if (framework && isJsFramework(framework)) {
		switch (framework) {
			case 'react':
				return `import { useTraceway } from "@tracewayapp/react";

// In a component using the hook
const { captureException } = useTraceway();
captureException(new Error("Test error"));`;
			case 'svelte':
				return `import { getTraceway } from "@tracewayapp/svelte";

const { captureException } = getTraceway();
captureException(new Error("Test error"));`;
			case 'vuejs':
				return `import { useTraceway } from "@tracewayapp/vue";

const { captureException } = useTraceway();
captureException(new Error("Test error"));`;
			case 'jquery':
				return `import { captureException } from "@tracewayapp/jquery";

captureException(new Error("Test error"));`;
			case 'react-native':
				return `import { useTraceway } from "@tracewayapp/react-native";

// In a component using the hook
const { captureException } = useTraceway();
captureException(new Error("Test error"));`;
			default:
				return `import { captureException } from "@tracewayapp/${getPackageName(framework)}";

captureException(new Error("Test error"));`;
		}
	}
	return '';
}

function getPackageName(framework: Framework): string {
	switch (framework) {
		case 'react':
			return 'react';
		case 'svelte':
			return 'svelte';
		case 'vuejs':
			return 'vue';
		case 'nextjs':
			return 'next';
		case 'nestjs':
			return 'nest';
		case 'express':
			return 'express';
		case 'remix':
			return 'remix';
		case 'jquery':
			return 'jquery';
		case 'react-native':
			return 'react-native';
		default:
			return 'react';
	}
}

export function getFrameworkLabel(framework: Framework): string {
	const labels: Record<Framework, string> = {
		gin: 'Gin',
		fiber: 'Fiber',
		chi: 'Chi',
		fasthttp: 'FastHTTP',
		stdlib: 'Standard Library (net/http)',
		custom: 'Custom Integration',
		react: 'React',
		svelte: 'Svelte',
		vuejs: 'Vue.js',
		nextjs: 'Next.js',
		nestjs: 'NestJS',
		express: 'Express',
		remix: 'Remix',
		jquery: 'jQuery',
		'react-native': 'React Native',
		hono: 'Hono',
		cloudflare: 'Cloudflare',
		opentelemetry: 'OpenTelemetry',
		symfony: 'Symfony',
		laravel: 'Laravel',
		django: 'Django',
		flutter: 'Flutter',
		android: 'Android',
		ios: 'iOS'
	};
	return labels[framework] || framework;
}

export type CodeLanguage =
	| 'go'
	| 'javascript'
	| 'bash'
	| 'php'
	| 'python'
	| 'dart'
	| 'kotlin'
	| 'swift';

export function getCodeLanguage(framework: Framework): CodeLanguage {
	if (framework === 'symfony') return 'php';
	if (framework === 'laravel') return 'php';
	if (framework === 'django') return 'python';
	if (framework === 'opentelemetry') return 'go';
	if (framework === 'hono') return 'javascript';
	if (framework === 'cloudflare') return 'javascript';
	if (framework === 'flutter') return 'dart';
	if (framework === 'android') return 'kotlin';
	if (framework === 'ios') return 'swift';
	return isJsFramework(framework) ? 'javascript' : 'go';
}

/** The install line is Gradle Kotlin on Android and a Package.swift entry on iOS; a shell command elsewhere. */
export function getInstallLanguage(framework: Framework): 'bash' | 'kotlin' | 'swift' {
	if (framework === 'android') return 'kotlin';
	if (framework === 'ios') return 'swift';
	return 'bash';
}
