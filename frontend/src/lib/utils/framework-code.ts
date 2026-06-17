import type { Framework } from '$lib/state/projects.svelte';
import { isJsFramework } from '$lib/state/projects.svelte';

export function getInstallCommand(framework: Framework): string {
	const base = 'go get go.tracewayapp.com';
	switch (framework) {
		case 'gin':
			return `${base} && go get go.tracewayapp.com/tracewaygin`;
		case 'chi':
			return `${base} && go get go.tracewayapp.com/tracewaychi`;
		case 'fiber':
			return `${base} && go get go.tracewayapp.com/tracewayfiber`;
		case 'fasthttp':
			return `${base} && go get go.tracewayapp.com/tracewayfasthttp`;
		case 'stdlib':
			return `${base} && go get go.tracewayapp.com/tracewayhttp`;
		case 'react':
			return 'npm install @tracewayapp/react';
		case 'svelte':
			return 'npm install @tracewayapp/svelte';
		case 'vuejs':
			return 'npm install @tracewayapp/vue';
		case 'nextjs':
			return 'npm install @tracewayapp/react';
		case 'nestjs':
			return 'npm install @tracewayapp/nest';
		case 'express':
			return 'npm install @tracewayapp/express';
		case 'remix':
			return 'npm install @tracewayapp/remix';
		case 'jquery':
			return 'npm install @tracewayapp/jquery';
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
			return 'flutter pub add traceway';
		case 'android':
			return 'implementation("com.tracewayapp:traceway:1.0.0")';
		case 'ios':
			return '.package(url: "https://github.com/tracewayapp/traceway-ios.git", from: "0.1.0")';
		case 'custom':
		default:
			return base;
	}
}

export function getFrameworkCode(framework: Framework, token: string, backendUrl: string): string {
	const connectionString = token
		? `${token}@${backendUrl}/api/report`
		: `YOUR_TOKEN@${backendUrl}/api/report`;

	switch (framework) {
		case 'gin':
			return `package main

import (
    "github.com/gin-gonic/gin"
    tracewaygin "go.tracewayapp.com/tracewaygin"
)

func main() {
    r := gin.Default()
    r.Use(tracewaygin.New("${connectionString}"))
    r.Run(":8080")
}`;

		case 'chi':
			return `package main

import (
    "net/http"

    "github.com/go-chi/chi/v5"
    tracewaychi "go.tracewayapp.com/tracewaychi"
)

func main() {
    r := chi.NewRouter()
    r.Use(tracewaychi.New("${connectionString}"))

    r.Get("/api/users", getUsers)
    http.ListenAndServe(":8080", r)
}`;

		case 'fiber':
			return `package main

import (
    "github.com/gofiber/fiber/v2"
    tracewayfiber "go.tracewayapp.com/tracewayfiber"
)

func main() {
    app := fiber.New()
    app.Use(tracewayfiber.New("${connectionString}"))

    app.Get("/api/users", getUsers)
    app.Listen(":8080")
}`;

		case 'fasthttp':
			return `package main

import (
    "github.com/valyala/fasthttp"
    tracewayfasthttp "go.tracewayapp.com/tracewayfasthttp"
)

func main() {
    handler := func(ctx *fasthttp.RequestCtx) {
        ctx.SetStatusCode(200)
        ctx.SetBodyString("Hello, World!")
    }

    tracedHandler := tracewayfasthttp.New("${connectionString}")(handler)
    fasthttp.ListenAndServe(":8080", tracedHandler)
}`;

		case 'stdlib':
			return `package main

import (
    "net/http"

    tracewayhttp "go.tracewayapp.com/tracewayhttp"
)

func main() {
    mux := http.NewServeMux()
    mux.HandleFunc("/api/users", getUsers)

    handler := tracewayhttp.New("${connectionString}")(mux)
    http.ListenAndServe(":8080", handler)
}`;

		case 'react':
			return `import { TracewayProvider } from "@tracewayapp/react";

function App() {
  return (
    <TracewayProvider connectionString="${connectionString}">
      <YourApp />
    </TracewayProvider>
  );
}

export default App;`;

		case 'svelte':
			return `<!-- src/routes/+layout.svelte -->
<script>
  import { setupTraceway } from "@tracewayapp/svelte";
  import { browser } from "$app/environment";

  if (browser) {
    setupTraceway({
      connectionString: "${connectionString}",
    });
  }
</script>

<slot />`;

		case 'vuejs':
			return `import { createApp } from "vue";
import { createTracewayPlugin } from "@tracewayapp/vue";
import App from "./App.vue";

const app = createApp(App);

app.use(createTracewayPlugin({
  connectionString: "${connectionString}",
}));

app.mount("#app");`;

		case 'nextjs':
			return `// app/traceway-provider.tsx
"use client";

import { TracewayProvider } from "@tracewayapp/react";

export function TracewayClientProvider({ children }: { children: React.ReactNode }) {
  return (
    <TracewayProvider connectionString="${connectionString}">
      {children}
    </TracewayProvider>
  );
}

// app/layout.tsx
import { TracewayClientProvider } from "./traceway-provider";

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html>
      <body>
        <TracewayClientProvider>{children}</TracewayClientProvider>
      </body>
    </html>
  );
}`;

		case 'nestjs':
			return `import { Module } from "@nestjs/common";
import { TracewayModule } from "@tracewayapp/nest";

@Module({
    imports: [
        TracewayModule.forRoot({
            connectionString: "${connectionString}",
        }),
    ],
})
export class AppModule {}`;

		case 'express':
			return `import express from "express";
import { traceway } from "@tracewayapp/express";

const app = express();
app.use(traceway("${connectionString}"));

app.get("/api/users", (req, res) => {
    res.json({ users: [] });
});

app.listen(8080);`;

		case 'remix':
			return `import { withTraceway } from "@tracewayapp/remix";

export default withTraceway({
    connectionString: "${connectionString}",
});`;

		case 'jquery':
			return `import { init } from "@tracewayapp/jquery";

init("${connectionString}");

// jQuery AJAX errors are captured automatically
// Distributed trace headers are injected into $.ajax() requests`;

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
// .env  — point the OTLP exporter at Traceway
//
// OTEL_SERVICE_NAME=my-laravel-app
// OTEL_TRACES_EXPORTER=otlp
// OTEL_METRICS_EXPORTER=otlp
// OTEL_LOGS_EXPORTER=otlp
// OTEL_EXPORTER_OTLP_PROTOCOL=http/json
// OTEL_EXPORTER_OTLP_ENDPOINT=${backendUrl}/api/otel
// OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer ${token || 'YOUR_TOKEN'}"
//
// Optional: send Laravel logs to Traceway via the auto-injected 'otlp' channel
// LOG_CHANNEL=otlp

// That's it — keepsuit/laravel-opentelemetry's service provider auto-registers
// TraceRequestMiddleware as a global middleware, so every HTTP request, DB query,
// queued job, Redis call, cache op, view render and outbound Http:: call is
// traced automatically. Open config/opentelemetry.php to tune which
// instrumentations are enabled.`;

		case 'django':
			return `# .env  — point the OTLP exporter at Traceway
#
# OTEL_SERVICE_NAME=my-django-app
# OTEL_TRACES_EXPORTER=otlp
# OTEL_METRICS_EXPORTER=otlp
# OTEL_LOGS_EXPORTER=otlp
# OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf
# OTEL_EXPORTER_OTLP_ENDPOINT=${backendUrl}/api/otel
# OTEL_EXPORTER_OTLP_HEADERS=Authorization=Bearer%20${token || 'YOUR_TOKEN'}
# OTEL_PYTHON_LOGGING_AUTO_INSTRUMENTATION_ENABLED=true

# Then launch Django through the OTel agent — no code changes needed:
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

		case 'flutter':
			return `import 'package:flutter/material.dart';
import 'package:traceway/traceway.dart';

void main() {
  Traceway.run(
    connectionString: '${connectionString}',
    options: TracewayOptions(
      screenCapture: true,
      version: '1.0.0',
    ),
    child: MyApp(),
  );
}`;

		case 'android':
			return `import android.app.Application
import com.tracewayapp.traceway.Traceway
import com.tracewayapp.traceway.TracewayOptions

class MyApp : Application() {
    override fun onCreate() {
        super.onCreate()
        Traceway.init(
            application = this,
            connectionString = "${connectionString}",
            options = TracewayOptions(version = "1.0.0"),
        )
    }
}`;

		case 'ios':
			return `import SwiftUI
import Traceway

@main
struct MyApp: App {
    init() {
        Traceway.start(
            connectionString: "${connectionString}",
            options: TracewayOptions(version: "1.0.0")
        )
    }

    var body: some Scene {
        WindowGroup {
            ContentView()
        }
    }
}`;

		case 'custom':
		default:
			return `package main

import (
    "go.tracewayapp.com"
)

func main() {
    traceway.Init(
        "${connectionString}",
        traceway.WithVersion("1.0.0"),
        traceway.WithServerName("my-server"),
    )
}`;
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
		return `// Trigger a test error
fatalError("Test error from Traceway integration")`;
	}
	if (framework && isJsFramework(framework)) {
		return `// Trigger a test error
throw new Error("Test error from Traceway integration");`;
	}
	return `r.GET("/testing", func(c *gin.Context) {
    panic("Test error from Traceway integration")
})`;
}

export function getTestingRouteCode2(framework?: Framework): string {
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

TracewayClient.instance?.captureException(
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
    Traceway.capture(error)
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
			case 'nextjs':
				return `import { useTraceway } from "@tracewayapp/react";

// In a client component
"use client";
const { captureException } = useTraceway();
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
	return `r.GET("/testing", func(c *gin.Context) {
    c.AbortWithError(500, traceway.NewStackTraceErrorf("testing"))
})`;
}

function getPackageName(framework: Framework): string {
	switch (framework) {
		case 'react': return 'react';
		case 'svelte': return 'svelte';
		case 'vuejs': return 'vue';
		case 'nextjs': return 'next';
		case 'nestjs': return 'nest';
		case 'express': return 'express';
		case 'remix': return 'remix';
		case 'jquery': return 'jquery';
		case 'react-native': return 'react-native';
		default: return 'react';
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
		ios: 'iOS',
	};
	return labels[framework] || framework;
}

export function getCodeLanguage(framework: Framework): 'go' | 'javascript' | 'bash' | 'php' | 'python' {
	if (framework === 'symfony') return 'php';
	if (framework === 'laravel') return 'php';
	if (framework === 'django') return 'python';
	if (framework === 'opentelemetry') return 'go';
	if (framework === 'hono') return 'javascript';
	if (framework === 'cloudflare') return 'javascript';
	if (framework === 'flutter') return 'javascript'; // closest to Dart syntax highlighting
	if (framework === 'android') return 'javascript'; // closest to Kotlin syntax highlighting
	if (framework === 'ios') return 'javascript';
	return isJsFramework(framework) ? 'javascript' : 'go';
}
