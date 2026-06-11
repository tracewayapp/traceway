export type OtelTargetId =
	| 'collector'
	| 'nodejs'
	| 'go'
	| 'python'
	| 'java'
	| 'dotnet'
	| 'php'
	| 'ruby'
	| 'other';

export type OtelFramework = {
	id: string;
	label: string;
};

export type OtelTarget = {
	id: OtelTargetId;
	label: string;
	frameworks: OtelFramework[];
};

export type OtelStepLanguage =
	| 'bash'
	| 'go'
	| 'javascript'
	| 'typescript'
	| 'python'
	| 'gradle'
	| 'csharp'
	| 'ruby'
	| 'yaml';

export type OtelStep = {
	title: string;
	description?: string;
	code?: string;
	codeLanguage?: OtelStepLanguage;
	link?: { label: string; href: string };
};

export const OTEL_TARGETS: OtelTarget[] = [
	{ id: 'collector', label: 'Collector', frameworks: [] },
	{
		id: 'nodejs',
		label: 'Node.js',
		frameworks: [
			{ id: 'express', label: 'Express' },
			{ id: 'nestjs', label: 'NestJS' },
			{ id: 'fastify', label: 'Fastify' },
			{ id: 'nextjs', label: 'Next.js' },
			{ id: 'koa', label: 'Koa' },
			{ id: 'other', label: 'Other' }
		]
	},
	{
		id: 'go',
		label: 'Go',
		frameworks: [
			{ id: 'gin', label: 'Gin' },
			{ id: 'echo', label: 'Echo' },
			{ id: 'chi', label: 'Chi' },
			{ id: 'fiber', label: 'Fiber' },
			{ id: 'mux', label: 'gorilla/mux' },
			{ id: 'nethttp', label: 'net/http' }
		]
	},
	{
		id: 'python',
		label: 'Python',
		frameworks: [
			{ id: 'django', label: 'Django' },
			{ id: 'flask', label: 'Flask' },
			{ id: 'fastapi', label: 'FastAPI' },
			{ id: 'other', label: 'Other' }
		]
	},
	{
		id: 'java',
		label: 'Java',
		frameworks: [
			{ id: 'agent', label: 'Any framework' },
			{ id: 'spring', label: 'Spring Boot' }
		]
	},
	{ id: 'dotnet', label: '.NET', frameworks: [] },
	{
		id: 'php',
		label: 'PHP',
		frameworks: [
			{ id: 'symfony', label: 'Symfony' },
			{ id: 'laravel', label: 'Laravel' },
			{ id: 'slim', label: 'Slim' },
			{ id: 'other', label: 'Other' }
		]
	},
	{
		id: 'ruby',
		label: 'Ruby',
		frameworks: [
			{ id: 'rails', label: 'Rails' },
			{ id: 'other', label: 'Other' }
		]
	},
	{ id: 'other', label: 'Other', frameworks: [] }
];

function envBlock(backendUrl: string, token: string, extra: string[] = []): string {
	return [
		'OTEL_SERVICE_NAME=my-service',
		`OTEL_EXPORTER_OTLP_ENDPOINT=${backendUrl}/api/otel`,
		`OTEL_EXPORTER_OTLP_HEADERS=Authorization=Bearer ${token}`,
		...extra
	].join('\n');
}

function envStep(backendUrl: string, token: string, extra: string[] = [], note = ''): OtelStep {
	return {
		title: 'Configure the Exporter',
		description: `Set these environment variables in your shell, .env file, or deployment config. The SDK appends /v1/traces and /v1/metrics to the endpoint automatically.${note ? ' ' + note : ''}`,
		code: envBlock(backendUrl, token, extra),
		codeLanguage: 'bash'
	};
}

function collectorConfig(backendUrl: string, token: string): string {
	return `exporters:
  otlphttp:
    endpoint: "${backendUrl}/api/otel"
    headers:
      Authorization: "Bearer ${token}"

service:
  pipelines:
    traces:
      exporters: [otlphttp]
    metrics:
      exporters: [otlphttp]`;
}

const GO_BOOTSTRAP = `import (
	"context"
	"log"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func initTracer(ctx context.Context) *sdktrace.TracerProvider {
	exp, err := otlptracehttp.New(ctx)
	if err != nil {
		log.Fatal(err)
	}
	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exp))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	return tp
}`;

const GO_FRAMEWORKS: Record<string, { lib: string; snippet: string; note?: string }> = {
	gin: {
		lib: 'go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin',
		snippet: `import "go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"

r := gin.Default()
r.Use(otelgin.Middleware("my-service"))`
	},
	echo: {
		lib: 'go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho',
		snippet: `import "go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho"

e := echo.New()
e.Use(otelecho.Middleware("my-service"))`
	},
	chi: {
		lib: 'github.com/riandyrn/otelchi',
		snippet: `import "github.com/riandyrn/otelchi"

r := chi.NewRouter()
r.Use(otelchi.Middleware("my-service", otelchi.WithChiRoutes(r)))`,
		note: 'WithChiRoutes lets the middleware resolve the route pattern so endpoints group correctly.'
	},
	fiber: {
		lib: 'github.com/gofiber/contrib/v3/otel',
		snippet: `import fiberotel "github.com/gofiber/contrib/v3/otel"

app := fiber.New()
app.Use(fiberotel.Middleware())`,
		note: 'For Fiber v2 use github.com/gofiber/contrib/otelfiber/v2 and otelfiber.Middleware() instead.'
	},
	mux: {
		lib: 'go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux',
		snippet: `import "go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux"

r := mux.NewRouter()
r.Use(otelmux.Middleware("my-service"))`
	},
	nethttp: {
		lib: 'go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp',
		snippet: `import "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

mux := http.NewServeMux()
mux.Handle("GET /users/{id}", otelhttp.NewHandler(http.HandlerFunc(getUser), "GET /users/{id}"))
http.ListenAndServe(":8080", mux)`,
		note: 'Wrap each route individually with Go 1.22+ method patterns so the route is set on spans and endpoints group by pattern instead of raw URL.'
	}
};

const NODE_ZERO_CODE_INSTALL = 'npm install @opentelemetry/api @opentelemetry/auto-instrumentations-node';

function nodeZeroCodeSteps(
	backendUrl: string,
	token: string,
	entrypoint: string,
	installNote: string,
	runNote: string
): OtelStep[] {
	return [
		{
			title: 'Install the SDK',
			description: installNote,
			code: NODE_ZERO_CODE_INSTALL,
			codeLanguage: 'bash'
		},
		envStep(backendUrl, token),
		{
			title: 'Run with Instrumentation',
			description: runNote,
			code: `node --require @opentelemetry/auto-instrumentations-node/register ${entrypoint}`,
			codeLanguage: 'bash'
		}
	];
}

export function getOtelSteps(
	target: OtelTargetId,
	framework: string,
	backendUrl: string,
	token: string
): OtelStep[] {
	switch (target) {
		case 'collector':
			return [
				{
					title: 'Add the Traceway Exporter',
					description:
						'Merge this into your OpenTelemetry Collector configuration. Any pipeline that lists the otlphttp exporter will be forwarded to Traceway.',
					code: collectorConfig(backendUrl, token),
					codeLanguage: 'yaml'
				},
				{
					title: 'Restart the Collector',
					description:
						'Restart the Collector to apply the configuration. Traces and metrics flowing through its pipelines will appear in Traceway.'
				}
			];

		case 'nodejs': {
			if (framework === 'fastify') {
				return [
					{
						title: 'Install the SDK',
						description:
							'Fastify is instrumented by the @fastify/otel package maintained by the Fastify team.',
						code: 'npm install @opentelemetry/api @opentelemetry/sdk-node @opentelemetry/auto-instrumentations-node @fastify/otel',
						codeLanguage: 'bash'
					},
					{
						title: 'Create instrumentation.js',
						description: 'Add this file at the project root.',
						code: `const { NodeSDK } = require('@opentelemetry/sdk-node');
const { getNodeAutoInstrumentations } = require('@opentelemetry/auto-instrumentations-node');
const { FastifyOtelInstrumentation } = require('@fastify/otel');

new NodeSDK({
  instrumentations: [
    getNodeAutoInstrumentations(),
    new FastifyOtelInstrumentation({ registerOnInitialization: true }),
  ],
}).start();`,
						codeLanguage: 'javascript'
					},
					envStep(backendUrl, token),
					{
						title: 'Run with Instrumentation',
						code: 'node --require ./instrumentation.js app.js',
						codeLanguage: 'bash'
					}
				];
			}
			if (framework === 'nextjs') {
				return [
					{
						title: 'Install the SDK',
						code: 'npm install @vercel/otel',
						codeLanguage: 'bash'
					},
					{
						title: 'Create instrumentation.ts',
						description:
							'Add this file at the project root (next to package.json). Next.js calls register() automatically on startup.',
						code: `import { registerOTel } from '@vercel/otel'

export function register() {
  registerOTel({ serviceName: 'my-service' })
}`,
						codeLanguage: 'typescript'
					},
					envStep(
						backendUrl,
						token,
						[],
						'Start your app normally with next start; no extra flags are needed.'
					)
				];
			}
			if (framework === 'nestjs') {
				return nodeZeroCodeSteps(
					backendUrl,
					token,
					'dist/main.js',
					'Auto-instrumentation captures NestJS routes, status codes, and errors through the default Express adapter with no code changes. If you use the Fastify adapter, follow the Fastify setup instead.',
					'Routes group by pattern automatically.'
				);
			}
			if (framework === 'koa') {
				return nodeZeroCodeSteps(
					backendUrl,
					token,
					'app.js',
					'Auto-instrumentation captures Koa requests, status codes, and errors with no code changes.',
					'Route patterns are captured when routing with @koa/router.'
				);
			}
			return nodeZeroCodeSteps(
				backendUrl,
				token,
				'app.js',
				'Auto-instrumentation captures routes, status codes, and errors with no code changes.',
				'For ESM apps, add --experimental-loader=@opentelemetry/instrumentation/hook.mjs and use --import instead of --require.'
			);
		}

		case 'go': {
			const fw = GO_FRAMEWORKS[framework] ?? GO_FRAMEWORKS.gin;
			return [
				{
					title: 'Install the SDK',
					code: `go get go.opentelemetry.io/otel go.opentelemetry.io/otel/sdk go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp ${fw.lib}`,
					codeLanguage: 'bash'
				},
				{
					title: 'Initialize the SDK',
					description:
						'Call initTracer at startup and defer tp.Shutdown(ctx) before exit. The exporter reads the environment variables from the next step.',
					code: GO_BOOTSTRAP,
					codeLanguage: 'go'
				},
				{
					title: 'Add the Middleware',
					description: fw.note,
					code: fw.snippet,
					codeLanguage: 'go'
				},
				envStep(backendUrl, token)
			];
		}

		case 'python': {
			const runByFramework: Record<string, { cmd: string; note?: string }> = {
				django: {
					cmd: 'opentelemetry-instrument python manage.py runserver --noreload',
					note: 'The --noreload flag is required with runserver; the autoreloader breaks instrumentation. It is not needed under gunicorn or other production servers.'
				},
				flask: { cmd: 'opentelemetry-instrument flask run' },
				fastapi: {
					cmd: 'opentelemetry-instrument uvicorn main:app',
					note: 'Avoid --reload and --workers with zero-code instrumentation; for multi-worker production use gunicorn with uvicorn workers.'
				},
				other: { cmd: 'opentelemetry-instrument python app.py' }
			};
			const run = runByFramework[framework] ?? runByFramework.other;
			return [
				{
					title: 'Install the SDK',
					description:
						'opentelemetry-bootstrap detects your installed packages and adds the matching instrumentation.',
					code: 'pip install opentelemetry-distro opentelemetry-exporter-otlp-proto-http\nopentelemetry-bootstrap -a install',
					codeLanguage: 'bash'
				},
				envStep(
					backendUrl,
					token,
					['OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf'],
					'The protocol variable is required; the Python SDK defaults to gRPC.'
				),
				{
					title: 'Run with Instrumentation',
					description: run.note,
					code: run.cmd,
					codeLanguage: 'bash'
				}
			];
		}

		case 'java': {
			if (framework === 'spring') {
				return [
					{
						title: 'Add the Starter',
						description: 'Add the OpenTelemetry Spring Boot starter to your Gradle build (a Maven dependency works the same way).',
						code: `implementation(platform("io.opentelemetry.instrumentation:opentelemetry-instrumentation-bom:2.28.1"))
implementation("io.opentelemetry.instrumentation:opentelemetry-spring-boot-starter")`,
						codeLanguage: 'gradle'
					},
					envStep(
						backendUrl,
						token,
						[],
						'Start your app normally; the starter reads these variables and reports routes, status codes, and exceptions.'
					)
				];
			}
			return [
				{
					title: 'Download the Java Agent',
					description:
						'The agent instruments Spring, JAX-RS, and most Java frameworks with zero code changes.',
					code: 'curl -L -O https://github.com/open-telemetry/opentelemetry-java-instrumentation/releases/latest/download/opentelemetry-javaagent.jar',
					codeLanguage: 'bash'
				},
				envStep(backendUrl, token),
				{
					title: 'Run with the Agent',
					code: 'java -javaagent:./opentelemetry-javaagent.jar -jar myapp.jar',
					codeLanguage: 'bash'
				}
			];
		}

		case 'dotnet':
			return [
				{
					title: 'Install the Packages',
					code: `dotnet add package OpenTelemetry.Extensions.Hosting
dotnet add package OpenTelemetry.Instrumentation.AspNetCore
dotnet add package OpenTelemetry.Exporter.OpenTelemetryProtocol`,
					codeLanguage: 'bash'
				},
				{
					title: 'Add to Program.cs',
					description:
						'Keep AddOtlpExporter() empty so the exporter is driven entirely by the environment variables in the next step.',
					code: `builder.Services.AddOpenTelemetry()
    .WithTracing(t => t
        .AddAspNetCoreInstrumentation()
        .AddOtlpExporter());`,
					codeLanguage: 'csharp'
				},
				envStep(
					backendUrl,
					token,
					['OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf'],
					'The protocol variable is required; the .NET exporter defaults to gRPC.'
				)
			];

		case 'php': {
			const autoPackages: Record<string, string> = {
				symfony: ' open-telemetry/opentelemetry-auto-symfony',
				laravel: ' open-telemetry/opentelemetry-auto-laravel',
				slim: ' open-telemetry/opentelemetry-auto-slim',
				other: ''
			};
			const autoPackage = autoPackages[framework] ?? '';
			return [
				{
					title: 'Install the SDK',
					description:
						'Auto-instrumentation needs the opentelemetry PECL extension; enable it with extension=opentelemetry in php.ini.' +
						(framework === 'other'
							? ' Find auto-instrumentation packages for your framework in the OpenTelemetry registry.'
							: ''),
					code: `pecl install opentelemetry
composer require open-telemetry/sdk open-telemetry/exporter-otlp php-http/guzzle7-adapter${autoPackage}`,
					codeLanguage: 'bash',
					link:
						framework === 'other'
							? {
									label: 'Browse PHP instrumentation packages',
									href: 'https://opentelemetry.io/ecosystem/registry/?language=php&component=instrumentation'
								}
							: undefined
				},
				envStep(
					backendUrl,
					token,
					['OTEL_PHP_AUTOLOAD_ENABLED=true'],
					'These must be real process environment variables; the extension does not read framework .env files. Use env[...] in php-fpm pool config or SetEnv in Apache.'
				)
			];
		}

		case 'ruby': {
			if (framework === 'rails') {
				return [
					{
						title: 'Install the Gems',
						code: 'bundle add opentelemetry-sdk opentelemetry-exporter-otlp opentelemetry-instrumentation-rails',
						codeLanguage: 'bash'
					},
					{
						title: 'Create the Initializer',
						description: 'Add config/initializers/opentelemetry.rb.',
						code: `require 'opentelemetry/sdk'
require 'opentelemetry/exporter/otlp'
require 'opentelemetry/instrumentation/rails'

OpenTelemetry::SDK.configure do |c|
  c.use 'OpenTelemetry::Instrumentation::Rails'
end`,
						codeLanguage: 'ruby'
					},
					envStep(backendUrl, token)
				];
			}
			return [
				{
					title: 'Install the Gems',
					code: 'bundle add opentelemetry-sdk opentelemetry-exporter-otlp opentelemetry-instrumentation-all',
					codeLanguage: 'bash'
				},
				{
					title: 'Configure the SDK',
					description: 'Run this once at startup, before your app starts handling requests.',
					code: `require 'opentelemetry/sdk'
require 'opentelemetry/exporter/otlp'
require 'opentelemetry/instrumentation/all'

OpenTelemetry::SDK.configure do |c|
  c.use_all
end`,
					codeLanguage: 'ruby'
				},
				envStep(backendUrl, token)
			];
		}

		default:
			return [
				{
					title: 'Configure any OpenTelemetry SDK',
					description:
						'Any language with an OTLP/HTTP exporter works. Set these environment variables; the protocol variable matters for SDKs that default to gRPC. Make sure http.route is set on root server spans so endpoints group by route pattern, and use SpanKind CONSUMER for background jobs.',
					code: envBlock(backendUrl, token, ['OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf']),
					codeLanguage: 'bash',
					link: {
						label: 'View all supported languages',
						href: 'https://opentelemetry.io/docs/languages/'
					}
				}
			];
	}
}
