<script lang="ts">
	import {
		Card,
		CardContent,
		CardDescription,
		CardHeader,
		CardTitle
	} from '$lib/components/ui/card';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Copy, Check, Eye, EyeOff } from 'lucide-svelte';
	import { projectsState, type ProjectWithToken, type Framework } from '$lib/state/projects.svelte';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import FrameworkIcon from '$lib/components/framework-icon.svelte';
	import Highlight from 'svelte-highlight';
	import go from 'svelte-highlight/languages/go';
	import bash from 'svelte-highlight/languages/bash';
	import { themeState } from '$lib/state/theme.svelte';
	import 'svelte-highlight/styles/github-dark.css';

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

			projectsState
				.getProjectWithToken(projectId)
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
			setTimeout(() => (copiedToken = false), 2000);
		}
	}

	// Framework-specific code snippets
	function getFrameworkCode(framework: Framework, token: string): string {
		const apiUrl = import.meta.env.VITE_API_URL || 'http://localhost:8082';
		const connectionString = token
			? `${token}@${apiUrl}/api/report`
			: `YOUR_TOKEN@${apiUrl}/api/report`;

		switch (framework) {
			case 'gin':
				return `import "github.com/traceway-io/go-client/traceway_gin"

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

	function getInstallCommand(framework: Framework): string {
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
		projectWithToken ? getFrameworkCode(projectWithToken.framework, projectWithToken.token) : ''
	);

	const installCommand = $derived(
		projectWithToken
			? getInstallCommand(projectWithToken.framework)
			: 'go get github.com/traceway-io/go-client'
	);

	async function copyCode() {
		await navigator.clipboard.writeText(sdkCode);
		copiedCode = true;
		setTimeout(() => (copiedCode = false), 2000);
	}

	async function copyInstall() {
		await navigator.clipboard.writeText(installCommand);
		copiedInstall = true;
		setTimeout(() => (copiedInstall = false), 2000);
	}
</script>

<div class="space-y-4">
	<div>
		<h2 class="text-2xl font-bold tracking-tight">Connection</h2>
		<p class="text-muted-foreground">Connect your application to Traceway using the SDK</p>
	</div>

	{#if loading}
		<Card>
			<CardContent class="flex items-center justify-center py-12">
				<LoadingCircle size="lg" />
			</CardContent>
		</Card>
	{:else if error}
		<Card>
			<CardContent class="p-6">
				<p class="text-destructive">{error}</p>
			</CardContent>
		</Card>
	{:else if projectWithToken}
		<!-- <Card>
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
        </Card> -->

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
						<Button variant="outline" size="sm" onclick={copyCode}>
							{#if copiedCode}
								<Check class="mr-2 h-4 w-4 text-green-500" />
								Copied!
							{:else}
								<Copy class="mr-2 h-4 w-4" />
								Copy
							{/if}
						</Button>
					</div>
					<div
						class="overflow-x-auto rounded-lg text-sm {themeState.isDark
							? 'dark-code'
							: 'light-code'}"
					>
						<Highlight language={go} code={sdkCode} />
					</div>
				</div>
			</CardContent>
		</Card>

		<Card>
			<CardHeader>
				<CardTitle>Installation</CardTitle>
				<CardDescription>Install the required packages using go get.</CardDescription>
			</CardHeader>
			<CardContent>
				<div class="relative">
					<div class="absolute top-2 right-2 z-10">
						<Button variant="outline" size="sm" onclick={copyInstall}>
							{#if copiedInstall}
								<Check class="mr-2 h-4 w-4 text-green-500" />
								Copied!
							{:else}
								<Copy class="mr-2 h-4 w-4" />
								Copy
							{/if}
						</Button>
					</div>
					<div
						class="overflow-x-auto rounded-lg text-sm {themeState.isDark
							? 'dark-code'
							: 'light-code'}"
					>
						<Highlight language={bash} code={installCommand} />
					</div>
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

<style>
	/* Light theme - override dark theme defaults */
	:global(.light-code .hljs) {
		background: #f6f8fa;
		color: #24292e;
	}
	:global(.light-code .hljs-keyword),
	:global(.light-code .hljs-selector-tag) {
		color: #d73a49;
	}
	:global(.light-code .hljs-string),
	:global(.light-code .hljs-attr) {
		color: #032f62;
	}
	:global(.light-code .hljs-function),
	:global(.light-code .hljs-title) {
		color: #6f42c1;
	}
	:global(.light-code .hljs-comment) {
		color: #6a737d;
	}
	:global(.light-code .hljs-built_in) {
		color: #005cc5;
	}

	/* Dark theme - ensure dark styles apply */
	:global(.dark-code .hljs) {
		background: #0d1117;
		color: #c9d1d9;
	}
	:global(.dark-code .hljs-keyword),
	:global(.dark-code .hljs-selector-tag) {
		color: #ff7b72;
	}
	:global(.dark-code .hljs-string),
	:global(.dark-code .hljs-attr) {
		color: #a5d6ff;
	}
	:global(.dark-code .hljs-function),
	:global(.dark-code .hljs-title) {
		color: #d2a8ff;
	}
	:global(.dark-code .hljs-comment) {
		color: #8b949e;
	}
	:global(.dark-code .hljs-built_in) {
		color: #79c0ff;
	}
</style>
