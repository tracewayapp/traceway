package seed

// Stack fixes what one service's telemetry looks like: the instrumentation
// scope names its libraries really report, the SDK language, the database
// system, and which runtime metrics it emits. Names follow the OpenTelemetry
// semantic conventions as of 2026 (http.request.method, http.route,
// http.response.status_code, db.system.name, db.query.text, ...).
type Stack struct {
	Language    string // telemetry.sdk.language
	Framework   string // label only
	ServerScope string // scope of the HTTP SERVER span
	DBScope     string // scope of database CLIENT spans
	DBSystem    string // db.system.name
	HTTPClient  string // scope of outgoing HTTP CLIENT spans
	CacheScope  string // scope of cache (redis) spans
	Runtime     string // go | jvm | python | node | dotnet | php
	// StackFormat selects how exception.stacktrace is rendered.
	StackFormat string
}

// Go returns a Go service. web: gin | nethttp | chi. db: pgx | sql.
func Go(web, db string) Stack {
	s := Stack{Language: "go", Framework: "go/" + web, Runtime: "go", StackFormat: "go",
		HTTPClient: "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp",
		CacheScope: "github.com/redis/go-redis/extra/redisotel"}
	switch web {
	case "gin":
		s.ServerScope = "go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	case "chi":
		s.ServerScope = "github.com/riandyrn/otelchi"
	default:
		s.ServerScope = "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	}
	switch db {
	case "pgx":
		s.DBScope, s.DBSystem = "github.com/exaring/otelpgx", "postgresql"
	case "mysql":
		s.DBScope, s.DBSystem = "github.com/XSAM/otelsql", "mysql"
	default:
		s.DBScope, s.DBSystem = "github.com/XSAM/otelsql", "postgresql"
	}
	return s
}

// Python returns a Python service. web: fastapi | django | flask.
func Python(web, db string) Stack {
	s := Stack{Language: "python", Framework: "python/" + web, Runtime: "python", StackFormat: "python",
		HTTPClient: "opentelemetry.instrumentation.httpx", CacheScope: "opentelemetry.instrumentation.redis"}
	switch web {
	case "django":
		s.ServerScope = "opentelemetry.instrumentation.django"
	case "flask":
		s.ServerScope = "opentelemetry.instrumentation.flask"
	default:
		s.ServerScope = "opentelemetry.instrumentation.fastapi"
	}
	switch db {
	case "psycopg":
		s.DBScope, s.DBSystem = "opentelemetry.instrumentation.psycopg", "postgresql"
	default:
		s.DBScope, s.DBSystem = "opentelemetry.instrumentation.sqlalchemy", "postgresql"
	}
	return s
}

// Node returns a Node.js service. web: express | nestjs | fastify.
func Node(web, db string) Stack {
	s := Stack{Language: "nodejs", Framework: "node/" + web, Runtime: "node", StackFormat: "node",
		HTTPClient: "@opentelemetry/instrumentation-undici", CacheScope: "@opentelemetry/instrumentation-ioredis"}
	switch web {
	case "nestjs":
		s.ServerScope = "@opentelemetry/instrumentation-http"
	case "fastify":
		s.ServerScope = "@opentelemetry/instrumentation-fastify"
	default:
		s.ServerScope = "@opentelemetry/instrumentation-http"
	}
	switch db {
	case "mysql":
		s.DBScope, s.DBSystem = "@opentelemetry/instrumentation-mysql2", "mysql"
	default:
		s.DBScope, s.DBSystem = "@opentelemetry/instrumentation-pg", "postgresql"
	}
	return s
}

// Java returns a JVM service instrumented by the OpenTelemetry Java agent.
func Java(web string) Stack {
	return Stack{Language: "java", Framework: "java/" + web, Runtime: "jvm", StackFormat: "java",
		ServerScope: "io.opentelemetry.tomcat-10.0", DBScope: "io.opentelemetry.jdbc", DBSystem: "postgresql",
		HTTPClient: "io.opentelemetry.java-http-client", CacheScope: "io.opentelemetry.lettuce-5.1"}
}

// PHP returns a PHP service (Laravel auto-instrumentation).
func PHP(web string) Stack {
	return Stack{Language: "php", Framework: "php/" + web, Runtime: "php", StackFormat: "php",
		ServerScope: "io.opentelemetry.contrib.php.laravel", DBScope: "io.opentelemetry.contrib.php.pdo", DBSystem: "mysql",
		HTTPClient: "io.opentelemetry.contrib.php.guzzle", CacheScope: "io.opentelemetry.contrib.php.laravel"}
}

// DotNet returns an ASP.NET Core service.
func DotNet() Stack {
	return Stack{Language: "dotnet", Framework: "dotnet/aspnetcore", Runtime: "dotnet", StackFormat: "dotnet",
		ServerScope: "Microsoft.AspNetCore", DBScope: "Npgsql", DBSystem: "postgresql",
		HTTPClient: "System.Net.Http", CacheScope: "OpenTelemetry.Instrumentation.StackExchangeRedis"}
}

// Span kinds (OTLP enum values).
const (
	KindInternal = 1
	KindServer   = 2
	KindClient   = 3
	KindProducer = 4
	KindConsumer = 5
)

// Status codes (OTLP enum values).
const (
	StatusUnset = 0
	StatusOK    = 1
	StatusError = 2
)

// OTel log severity numbers.
const (
	SevDebug = 5
	SevInfo  = 9
	SevWarn  = 13
	SevError = 17
	SevFatal = 21
)

// SeverityText maps a severity number to its OTel short name.
func SeverityText(n int) string {
	switch {
	case n >= 21:
		return "FATAL"
	case n >= 17:
		return "ERROR"
	case n >= 13:
		return "WARN"
	case n >= 9:
		return "INFO"
	case n >= 5:
		return "DEBUG"
	default:
		return "TRACE"
	}
}
