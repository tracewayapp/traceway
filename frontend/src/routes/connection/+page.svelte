<script lang="ts">
	import {
		Card,
		CardContent,
		CardDescription,
		CardHeader,
		CardTitle
	} from '$lib/components/ui/card';
	import { Button } from '$lib/components/ui/button';
	import { Copy, Check, KeyRound } from 'lucide-svelte';
	import { projectsState, type ProjectWithToken, isJsFramework } from '$lib/state/projects.svelte';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import FrameworkIcon from '$lib/components/framework-icon.svelte';
	import Highlight from 'svelte-highlight';
	import go from 'svelte-highlight/languages/go';
	import javascript from 'svelte-highlight/languages/javascript';
	import bash from 'svelte-highlight/languages/bash';
	import { themeState } from '$lib/state/theme.svelte';
	import 'svelte-highlight/styles/github-dark.css';
	import {
		getFrameworkCode,
		getInstallCommand,
		getFrameworkLabel
	} from '$lib/utils/framework-code';

	let projectWithToken = $derived(projectsState.currentProject);
	let copiedCode = $state(false);
	let copiedInstall = $state(false);
	let copiedToken = $state(false);
	let copiedCommand = $state(false);
	let generatingToken = $state(false);

	const sdkCode = $derived(
		projectWithToken
			? getFrameworkCode(
					projectWithToken.framework,
					projectWithToken.token,
					projectWithToken.backendUrl
				)
			: ''
	);

	const installCommand = $derived(
		projectWithToken ? getInstallCommand(projectWithToken.framework) : 'go get go.tracewayapp.com'
	);

	const highlightLanguage = $derived(
		projectWithToken && isJsFramework(projectWithToken.framework) ? javascript : go
	);

	const isJs = $derived(projectWithToken ? isJsFramework(projectWithToken.framework) : false);
	const sourceMapToken = $derived(projectWithToken?.sourceMapToken ?? null);

	const uploadCommand = $derived(
		projectWithToken && sourceMapToken
			? `npx @tracewayapp/sourcemap-upload --url ${projectWithToken.backendUrl} --token ${sourceMapToken} --version YOUR_VERSION --directory dist/assets`
			: ''
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

	async function generateToken() {
		generatingToken = true;
		try {
			await projectsState.generateSourceMapToken();
		} finally {
			generatingToken = false;
		}
	}

	async function copyToken() {
		if (!sourceMapToken) return;
		await navigator.clipboard.writeText(sourceMapToken);
		copiedToken = true;
		setTimeout(() => (copiedToken = false), 2000);
	}

	async function copyUploadCommand() {
		await navigator.clipboard.writeText(uploadCommand);
		copiedCommand = true;
		setTimeout(() => (copiedCommand = false), 2000);
	}
</script>

<div class="space-y-4">
	<div>
		<h2 class="text-2xl font-bold tracking-tight">Connection</h2>
		<p class="text-muted-foreground">Connect your application to Traceway using the SDK</p>
	</div>

	{#if projectWithToken}
		<Card>
			<CardHeader>
				<CardTitle class="flex items-center gap-2">
					<FrameworkIcon framework={projectWithToken.framework} />
					{getFrameworkLabel(projectWithToken.framework)} Integration
				</CardTitle>
				<CardDescription>
					Add Traceway to your application with just a few lines of code.
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
						<Highlight language={highlightLanguage} code={sdkCode} />
					</div>
				</div>
			</CardContent>
		</Card>

		<Card>
			<CardHeader>
				<CardTitle>Installation</CardTitle>
				<CardDescription
					>Install the required packages{isJs ? '' : ' using go get'}.</CardDescription
				>
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
		{#if isJs}
			<Card>
				<CardHeader>
					<CardTitle class="flex items-center gap-2">
						<KeyRound class="h-5 w-5" />
						Source Map Upload
					</CardTitle>
					<CardDescription>
						Upload source maps to see original file names and line numbers in stack traces
						from minified code.
					</CardDescription>
				</CardHeader>
				<CardContent>
					{#if sourceMapToken}
						<div class="space-y-4">
							<div>
								<p class="text-sm font-medium mb-2">Upload Token</p>
								<div class="flex items-center gap-2">
									<code
										class="flex-1 rounded-md bg-muted px-3 py-2 text-sm font-mono break-all"
										>{sourceMapToken}</code
									>
									<Button variant="outline" size="sm" onclick={copyToken}>
										{#if copiedToken}
											<Check class="h-4 w-4 text-green-500" />
										{:else}
											<Copy class="h-4 w-4" />
										{/if}
									</Button>
								</div>
							</div>
							<div>
								<p class="text-sm font-medium mb-2">Usage</p>
								<div class="relative">
									<div class="absolute top-2 right-2 z-10">
										<Button variant="outline" size="sm" onclick={copyUploadCommand}>
											{#if copiedCommand}
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
										<Highlight language={bash} code={uploadCommand} />
									</div>
								</div>
							</div>
						</div>
					{:else}
						<p class="text-sm text-muted-foreground mb-4">
							Generate an upload token to start uploading source maps as part of your build
							process.
						</p>
						<Button onclick={generateToken} disabled={generatingToken}>
							{#if generatingToken}
								<LoadingCircle class="mr-2 h-4 w-4" />
								Generating...
							{:else}
								<KeyRound class="mr-2 h-4 w-4" />
								Generate Upload Token
							{/if}
						</Button>
					{/if}
				</CardContent>
			</Card>
		{/if}
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
	:global(.light-code .hljs-name) {
		color: #4ba3f7;
	}
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
