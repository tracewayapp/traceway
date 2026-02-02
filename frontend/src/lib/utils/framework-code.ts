import type { Framework } from '$lib/state/projects.svelte';

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

export function getTestingRouteCode(): string {
	return `r.GET("/testing", func(c *gin.Context) {
    panic("Test error from Traceway integration")
})`;
}

export function getTestingRouteCode2(): string {
	return `r.GET("/testing", func(c *gin.Context) {
    c.AbortWithError(500, traceway.NewStackTraceErrorf("testing"))
})`;
}

export function getFrameworkLabel(framework: Framework): string {
	const labels: Record<Framework, string> = {
		gin: 'Gin',
		fiber: 'Fiber',
		chi: 'Chi',
		fasthttp: 'FastHTTP',
		stdlib: 'Standard Library (net/http)',
		custom: 'Custom Integration'
	};
	return labels[framework] || framework;
}
