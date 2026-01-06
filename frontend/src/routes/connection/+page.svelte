<script lang="ts">
    import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '$lib/components/ui/card';
    import { Button } from '$lib/components/ui/button';
    import { Input } from '$lib/components/ui/input';
    import { Label } from '$lib/components/ui/label';
    import { Copy, Check, Eye, EyeOff } from 'lucide-svelte';
    import { projectsState, type ProjectWithToken, type Framework } from '$lib/state/projects.svelte';
    import { Skeleton } from '$lib/components/ui/skeleton';
    import FrameworkIcon from '$lib/components/framework-icon.svelte';

    let projectWithToken = $state<ProjectWithToken | null>(null);
    let loading = $state(true);
    let error = $state<string | null>(null);
    let copiedToken = $state(false);
    let copiedCode = $state(false);
    let copiedInstall = $state(false);
    let showToken = $state(false);

    // React to project changes
    $effect(() => {
        const projectId = projectsState.currentProjectId;
        if (projectId) {
            loading = true;
            error = null;
            projectWithToken = null;

            projectsState.getProjectWithToken(projectId)
                .then((project) => {
                    projectWithToken = project;
                })
                .catch((e) => {
                    error = e instanceof Error ? e.message : 'Failed to load project';
                })
                .finally(() => {
                    loading = false;
                });
        } else {
            loading = false;
            projectWithToken = null;
        }
    });

    async function copyToken() {
        if (projectWithToken?.token) {
            await navigator.clipboard.writeText(projectWithToken.token);
            copiedToken = true;
            setTimeout(() => copiedToken = false, 2000);
        }
    }

    // Framework-specific code snippets
    function getFrameworkCode(framework: Framework, token: string): string {
        const tokenValue = token || 'YOUR_PROJECT_TOKEN';

        switch (framework) {
            case 'gin':
                return `package main

import (
    "github.com/gin-gonic/gin"
    "github.com/tracewayapp/traceway-go"
)

func main() {
    // Initialize Traceway client
    tw := traceway.New(traceway.Config{
        Token:     "${tokenValue}",
        ServerURL: "http://localhost:8082", // Your Traceway server
    })
    defer tw.Close()

    // Create Gin router
    router := gin.Default()

    // Add Traceway middleware
    router.Use(tw.GinMiddleware())

    // Your routes
    router.GET("/", func(c *gin.Context) {
        c.JSON(200, gin.H{"message": "Hello World"})
    })

    router.Run(":8080")
}`;

            case 'fiber':
                return `package main

import (
    "github.com/gofiber/fiber/v2"
    "github.com/tracewayapp/traceway-go"
)

func main() {
    // Initialize Traceway client
    tw := traceway.New(traceway.Config{
        Token:     "${tokenValue}",
        ServerURL: "http://localhost:8082", // Your Traceway server
    })
    defer tw.Close()

    // Create Fiber app
    app := fiber.New()

    // Add Traceway middleware
    app.Use(tw.FiberMiddleware())

    // Your routes
    app.Get("/", func(c *fiber.Ctx) error {
        return c.JSON(fiber.Map{"message": "Hello World"})
    })

    app.Listen(":8080")
}`;

            case 'chi':
                return `package main

import (
    "encoding/json"
    "net/http"

    "github.com/go-chi/chi/v5"
    "github.com/tracewayapp/traceway-go"
)

func main() {
    // Initialize Traceway client
    tw := traceway.New(traceway.Config{
        Token:     "${tokenValue}",
        ServerURL: "http://localhost:8082", // Your Traceway server
    })
    defer tw.Close()

    // Create Chi router
    r := chi.NewRouter()

    // Add Traceway middleware
    r.Use(tw.ChiMiddleware())

    // Your routes
    r.Get("/", func(w http.ResponseWriter, r *http.Request) {
        json.NewEncoder(w).Encode(map[string]string{"message": "Hello World"})
    })

    http.ListenAndServe(":8080", r)
}`;

            case 'fasthttp':
                return `package main

import (
    "github.com/valyala/fasthttp"
    "github.com/tracewayapp/traceway-go"
)

func main() {
    // Initialize Traceway client
    tw := traceway.New(traceway.Config{
        Token:     "${tokenValue}",
        ServerURL: "http://localhost:8082", // Your Traceway server
    })
    defer tw.Close()

    // Request handler
    handler := func(ctx *fasthttp.RequestCtx) {
        ctx.SetContentType("application/json")
        ctx.WriteString(\`{"message": "Hello World"}\`)
    }

    // Wrap with Traceway middleware
    wrappedHandler := tw.FastHTTPMiddleware(handler)

    fasthttp.ListenAndServe(":8080", wrappedHandler)
}`;

            case 'stdlib':
                return `package main

import (
    "encoding/json"
    "net/http"

    "github.com/tracewayapp/traceway-go"
)

func main() {
    // Initialize Traceway client
    tw := traceway.New(traceway.Config{
        Token:     "${tokenValue}",
        ServerURL: "http://localhost:8082", // Your Traceway server
    })
    defer tw.Close()

    // Create a new ServeMux
    mux := http.NewServeMux()

    // Your routes
    mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        json.NewEncoder(w).Encode(map[string]string{"message": "Hello World"})
    })

    // Wrap with Traceway middleware
    handler := tw.HTTPMiddleware(mux)

    http.ListenAndServe(":8080", handler)
}`;

            case 'custom':
            default:
                return `package main

import (
    "github.com/tracewayapp/traceway-go"
)

func main() {
    // Initialize Traceway client
    tw := traceway.New(traceway.Config{
        Token:     "${tokenValue}",
        ServerURL: "http://localhost:8082", // Your Traceway server
    })
    defer tw.Close()

    // Manual instrumentation example
    // Use tw.StartTransaction() and tw.EndTransaction() to track requests
    // See documentation for your specific framework integration
}`;
        }
    }

    function getInstallCommand(framework: Framework): string {
        const base = 'go get github.com/tracewayapp/traceway-go';
        switch (framework) {
            case 'gin':
                return `${base}\ngo get github.com/gin-gonic/gin`;
            case 'fiber':
                return `${base}\ngo get github.com/gofiber/fiber/v2`;
            case 'chi':
                return `${base}\ngo get github.com/go-chi/chi/v5`;
            case 'fasthttp':
                return `${base}\ngo get github.com/valyala/fasthttp`;
            case 'stdlib':
            case 'custom':
            default:
                return base;
        }
    }

    function getFrameworkLabel(framework: Framework): string {
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

    const sdkCode = $derived(
        projectWithToken
            ? getFrameworkCode(projectWithToken.framework, projectWithToken.token)
            : ''
    );

    const installCommand = $derived(
        projectWithToken
            ? getInstallCommand(projectWithToken.framework)
            : 'go get github.com/tracewayapp/traceway-go'
    );

    async function copyCode() {
        await navigator.clipboard.writeText(sdkCode);
        copiedCode = true;
        setTimeout(() => copiedCode = false, 2000);
    }

    async function copyInstall() {
        await navigator.clipboard.writeText(installCommand);
        copiedInstall = true;
        setTimeout(() => copiedInstall = false, 2000);
    }
</script>

<div class="space-y-6">
    <div>
        <h2 class="text-3xl font-bold tracking-tight">Connection</h2>
        <p class="text-muted-foreground">
            Connect your application to Traceway using the SDK
        </p>
    </div>

    {#if loading}
        <Card>
            <CardHeader>
                <Skeleton class="h-6 w-32" />
                <Skeleton class="h-4 w-64" />
            </CardHeader>
            <CardContent class="space-y-4">
                <Skeleton class="h-10 w-full" />
            </CardContent>
        </Card>
    {:else if error}
        <Card>
            <CardContent class="p-6">
                <p class="text-destructive">{error}</p>
            </CardContent>
        </Card>
    {:else if projectWithToken}
        <Card>
            <CardHeader>
                <CardTitle>Project Token</CardTitle>
                <CardDescription>
                    Use this token to authenticate your application with Traceway.
                    Keep it secure and don't share it publicly.
                </CardDescription>
            </CardHeader>
            <CardContent class="space-y-4">
                <div class="space-y-2">
                    <Label>Token for {projectWithToken.name}</Label>
                    <div class="flex gap-2">
                        <div class="relative flex-1">
                            <Input
                                type={showToken ? 'text' : 'password'}
                                value={projectWithToken.token}
                                readonly
                                class="pr-20 font-mono text-sm"
                            />
                        </div>
                        <Button
                            variant="outline"
                            size="icon"
                            onclick={() => showToken = !showToken}
                            title={showToken ? 'Hide token' : 'Show token'}
                        >
                            {#if showToken}
                                <EyeOff class="h-4 w-4" />
                            {:else}
                                <Eye class="h-4 w-4" />
                            {/if}
                        </Button>
                        <Button
                            variant="outline"
                            size="icon"
                            onclick={copyToken}
                            title="Copy token"
                        >
                            {#if copiedToken}
                                <Check class="h-4 w-4 text-green-500" />
                            {:else}
                                <Copy class="h-4 w-4" />
                            {/if}
                        </Button>
                    </div>
                </div>
            </CardContent>
        </Card>

        <Card>
            <CardHeader>
                <CardTitle class="flex items-center gap-2">
                    <FrameworkIcon framework={projectWithToken.framework} />
                    {getFrameworkLabel(projectWithToken.framework)} Integration
                </CardTitle>
                <CardDescription>
                    Add Traceway to your Go application with just a few lines of code.
                </CardDescription>
            </CardHeader>
            <CardContent>
                <div class="relative">
                    <div class="absolute top-2 right-2 z-10">
                        <Button
                            variant="outline"
                            size="sm"
                            onclick={copyCode}
                        >
                            {#if copiedCode}
                                <Check class="h-4 w-4 mr-2 text-green-500" />
                                Copied!
                            {:else}
                                <Copy class="h-4 w-4 mr-2" />
                                Copy
                            {/if}
                        </Button>
                    </div>
                    <pre class="bg-muted p-4 rounded-lg overflow-x-auto text-sm font-mono leading-relaxed"><code class="language-go">{sdkCode}</code></pre>
                </div>
            </CardContent>
        </Card>

        <Card>
            <CardHeader>
                <CardTitle>Installation</CardTitle>
                <CardDescription>
                    Install the required packages using go get.
                </CardDescription>
            </CardHeader>
            <CardContent>
                <div class="relative">
                    <div class="absolute top-2 right-2 z-10">
                        <Button
                            variant="outline"
                            size="sm"
                            onclick={copyInstall}
                        >
                            {#if copiedInstall}
                                <Check class="h-4 w-4 mr-2 text-green-500" />
                                Copied!
                            {:else}
                                <Copy class="h-4 w-4 mr-2" />
                                Copy
                            {/if}
                        </Button>
                    </div>
                    <pre class="bg-muted p-4 rounded-lg overflow-x-auto text-sm font-mono">{installCommand}</pre>
                </div>
            </CardContent>
        </Card>
    {:else}
        <Card>
            <CardContent class="p-6 text-center">
                <p class="text-muted-foreground">
                    No project selected. Please select or create a project from the dropdown above.
                </p>
            </CardContent>
        </Card>
    {/if}
</div>
