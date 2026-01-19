import type { Framework } from '$lib/state/projects.svelte';

export function getInstallCommand(framework: Framework): string {
	const base = 'go get github.com/traceway-io/go-client';
	switch (framework) {
		case 'gin':
			return `${base}`;
		case 'fiber':
		case 'chi':
		case 'fasthttp':
		case 'stdlib':
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
    "github.com/traceway-io/go-client/traceway_gin"
)

func main() {
    r := gin.Default()
    r.Use(traceway_gin.New("${connectionString}"))
    r.Run(":8080")
}`;

		case 'fiber':
		case 'chi':
		case 'fasthttp':
		case 'stdlib':
		case 'custom':
		default:
			return `// This framework is not currently supported.
//
// We welcome contributions! Please visit our GitHub repository
// to help implement support for ${framework === 'custom' ? 'custom frameworks' : framework}:
//
// https://github.com/traceway-io/go-client
//
// In the meantime, you can use the core SDK directly:

package main

import (
    "github.com/traceway-io/go-client"
)

func main() {
    // Initialize Traceway
    traceway.Init(
        "${connectionString}",
        traceway.WithVersion("1.0.0"),
        traceway.WithServerName("my-server"),
    )

    // Capture exceptions manually
    // traceway.CaptureException(err)
    // traceway.CaptureExceptionWithContext(ctx, err)

    // Capture messages
    // traceway.CaptureMessage("Something happened")
    // traceway.CaptureMessageWithContext(ctx, "Something happened")
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
