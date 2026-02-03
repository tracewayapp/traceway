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
		case 'drift':
			return `${base} && go get go.tracewayapp.com/drift`;
		case 'react':
			return 'npm install @tracewayapp/react';
		case 'svelte':
			return 'npm install @tracewayapp/svelte';
		case 'vuejs':
			return 'npm install @tracewayapp/vue';
		case 'nextjs':
			return 'npm install @tracewayapp/next';
		case 'nestjs':
			return 'npm install @tracewayapp/nest';
		case 'express':
			return 'npm install @tracewayapp/express';
		case 'remix':
			return 'npm install @tracewayapp/remix';
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
			return `import { init, captureException } from "@tracewayapp/react";

init("${connectionString}");

function App() {
    return <div>My App</div>;
}

export default App;`;

		case 'svelte':
			return `import { init, captureException } from "@tracewayapp/svelte";

init("${connectionString}");`;

		case 'vuejs':
			return `import { createApp } from "vue";
import { init, captureException } from "@tracewayapp/vue";

init("${connectionString}");

const app = createApp(App);
app.mount("#app");`;

		case 'nextjs':
			return `import { withTraceway } from "@tracewayapp/next";

export default withTraceway({
    connectionString: "${connectionString}",
});`;

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
	if (framework && isJsFramework(framework)) {
		return `// Trigger a test error
throw new Error("Test error from Traceway integration");`;
	}
	return `r.GET("/testing", func(c *gin.Context) {
    panic("Test error from Traceway integration")
})`;
}

export function getTestingRouteCode2(framework?: Framework): string {
	if (framework && isJsFramework(framework)) {
		return `import { captureException } from "@tracewayapp/${getPackageName(framework)}";

captureException(new Error("Test error from Traceway integration"));`;
	}
	return `r.GET("/testing", func(c *gin.Context) {
    c.AbortWithError(500, traceway.NewStackTraceErrorf("testing"))
})`;
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
		remix: 'Remix'
	};
	return labels[framework] || framework;
}

export function getCodeLanguage(framework: Framework): 'go' | 'javascript' {
	return isJsFramework(framework) ? 'javascript' : 'go';
}
