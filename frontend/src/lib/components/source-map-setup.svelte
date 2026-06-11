<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Copy, Check, KeyRound, RefreshCw } from 'lucide-svelte';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import * as Tabs from '$lib/components/ui/tabs';
	import { toast } from 'svelte-sonner';
	import Highlight from 'svelte-highlight';
	import javascript from 'svelte-highlight/languages/javascript';
	import typescript from 'svelte-highlight/languages/typescript';
	import bash from 'svelte-highlight/languages/bash';
	import { themeState } from '$lib/state/theme.svelte';
	import { projectsState } from '$lib/state/projects.svelte';
	import { authState } from '$lib/state/auth.svelte';

	type Bundler = 'vite' | 'rollup' | 'webpack';

	const bundlerConfigs: Record<
		Bundler,
		{
			label: string;
			file: string;
			directory: string;
			language: typeof javascript | typeof typescript;
			code: string;
		}
	> = {
		vite: {
			label: 'Vite',
			file: 'vite.config.ts',
			directory: 'dist/assets',
			language: typescript,
			code: `import { defineConfig } from "vite";
import { tracewayDebugIds } from "@tracewayapp/bundler-plugin/vite";

export default defineConfig({
  build: {
    sourcemap: true,
  },
  plugins: [tracewayDebugIds()],
});`
		},
		rollup: {
			label: 'Rollup',
			file: 'rollup.config.js',
			directory: 'dist',
			language: javascript,
			code: `import { tracewayDebugIds } from "@tracewayapp/bundler-plugin/rollup";

export default {
  output: {
    sourcemap: true,
  },
  plugins: [tracewayDebugIds()],
};`
		},
		webpack: {
			label: 'webpack',
			file: 'webpack.config.js',
			directory: 'dist',
			language: javascript,
			code: `const {
  TracewayDebugIdsWebpackPlugin,
} = require("@tracewayapp/bundler-plugin/webpack");

module.exports = {
  devtool: "source-map",
  plugins: [new TracewayDebugIdsWebpackPlugin()],
};`
		}
	};

	let bundler = $state<Bundler>('vite');
	let generatingToken = $state(false);
	let copiedToken = $state(false);
	let copiedPluginInstall = $state(false);
	let copiedBundlerConfig = $state(false);
	let copiedCommand = $state(false);

	const pluginInstallCommand = 'npm install -D @tracewayapp/bundler-plugin';

	const project = $derived(projectsState.currentProject);
	const sourceMapToken = $derived(project?.sourceMapToken ?? null);
	const isReadonly = $derived(
		authState.getRoleForOrganization(project?.organizationId ?? 0) === 'readonly'
	);

	const showBundlerSetup = $derived(project?.framework !== 'react-native');

	const uploadCommand = $derived(
		project && sourceMapToken
			? `npx @tracewayapp/sourcemap-upload \\
  --url ${project.backendUrl} \\
  --token ${sourceMapToken} \\
  --directory ${showBundlerSetup ? bundlerConfigs[bundler].directory : 'dist'}`
			: ''
	);

	let regenerateDialogOpen = $state(false);

	async function generateToken() {
		generatingToken = true;
		try {
			await projectsState.generateSourceMapToken();
		} finally {
			generatingToken = false;
		}
	}

	async function confirmRegenerate() {
		generatingToken = true;
		try {
			await projectsState.generateSourceMapToken();
			regenerateDialogOpen = false;
			toast.success('Successfully regenerated the Upload Token', { position: 'top-center' });
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

	async function copyPluginInstall() {
		await navigator.clipboard.writeText(pluginInstallCommand);
		copiedPluginInstall = true;
		setTimeout(() => (copiedPluginInstall = false), 2000);
	}

	async function copyBundlerConfig() {
		await navigator.clipboard.writeText(bundlerConfigs[bundler].code);
		copiedBundlerConfig = true;
		setTimeout(() => (copiedBundlerConfig = false), 2000);
	}

	async function copyUploadCommand() {
		await navigator.clipboard.writeText(uploadCommand);
		copiedCommand = true;
		setTimeout(() => (copiedCommand = false), 2000);
	}
</script>

{#if sourceMapToken}
	<div class="space-y-6">
		<div>
			<p class="mb-2 text-sm font-medium">Upload Token</p>
			<div class="flex items-center gap-2">
				<code class="flex-1 rounded-md bg-muted px-3 py-2 font-mono text-sm break-all"
					>{sourceMapToken}</code
				>
				<Button variant="outline" size="sm" onclick={copyToken}>
					{#if copiedToken}
						<Check class="h-4 w-4 text-green-500" />
					{:else}
						<Copy class="h-4 w-4" />
					{/if}
				</Button>
				<Button
					variant="destructiveOutline"
					size="sm"
					onclick={() => (regenerateDialogOpen = true)}
				>
					<RefreshCw class="mr-2 h-4 w-4" />
					Regenerate
				</Button>
			</div>
		</div>
		{#if showBundlerSetup}
			<div>
				<p class="mb-2 text-sm font-medium">Step 1: Install the bundler plugin</p>
				<div class="relative">
					<div class="absolute top-2 right-2 z-10">
						<Button variant="outline" size="sm" onclick={copyPluginInstall}>
							{#if copiedPluginInstall}
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
						<Highlight language={bash} code={pluginInstallCommand} />
					</div>
				</div>
			</div>
			<div>
				<p class="mb-2 text-sm font-medium">Step 2: Add the plugin to your bundler</p>
				<Tabs.Root
					value={bundler}
					onValueChange={(v) => {
						if (v) bundler = v as Bundler;
					}}
				>
					<Tabs.List class="mb-2">
						{#each Object.entries(bundlerConfigs) as [value, config] (value)}
							<Tabs.Trigger {value}>{config.label}</Tabs.Trigger>
						{/each}
					</Tabs.List>
				</Tabs.Root>
				<p class="mb-2 font-mono text-xs text-muted-foreground">
					{bundlerConfigs[bundler].file}
				</p>
				<div class="relative">
					<div class="absolute top-2 right-2 z-10">
						<Button variant="outline" size="sm" onclick={copyBundlerConfig}>
							{#if copiedBundlerConfig}
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
						<Highlight
							language={bundlerConfigs[bundler].language}
							code={bundlerConfigs[bundler].code}
						/>
					</div>
				</div>
			</div>
		{/if}
		<div>
			<p class="mb-2 text-sm font-medium">
				{showBundlerSetup ? 'Step 3: Upload after your production build' : 'Usage'}
			</p>
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
{:else if isReadonly}
	<p class="text-sm text-muted-foreground">
		An upload token is required to upload source maps. Ask an organization admin to generate one
		from the Connection page.
	</p>
{:else}
	<div class="flex items-center justify-between gap-4">
		<p class="text-sm text-muted-foreground">
			Generate an upload token to start uploading source maps as part of your build process.
		</p>
		<Button variant="outline" size="sm" onclick={generateToken} disabled={generatingToken}>
			{#if generatingToken}
				<LoadingCircle class="mr-2 h-4 w-4" />
				Generating...
			{:else}
				<KeyRound class="mr-2 h-4 w-4" />
				Generate Upload Token
			{/if}
		</Button>
	</div>
{/if}

<AlertDialog.Root bind:open={regenerateDialogOpen}>
	<AlertDialog.Content interactOutsideBehavior="close">
		<AlertDialog.Header>
			<AlertDialog.Title>Regenerate Upload Token</AlertDialog.Title>
			<AlertDialog.Description>
				A new upload token will be issued for this project and the current one will stop working
				immediately.
			</AlertDialog.Description>
		</AlertDialog.Header>

		<div class="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2">
			<p class="text-sm">
				<span class="font-semibold text-destructive">Warning:</span>
				<span class="text-destructive/90"
					>Any build pipeline or CI job still using the current token will fail to upload source
					maps until it is updated with the new token.</span
				>
			</p>
		</div>

		<AlertDialog.Footer class="sm:justify-between">
			<Button
				variant="outline"
				onclick={() => (regenerateDialogOpen = false)}
				disabled={generatingToken}
			>
				Cancel
			</Button>
			<Button variant="destructive" onclick={confirmRegenerate} disabled={generatingToken}>
				{generatingToken ? 'Regenerating...' : 'Regenerate Token'}
			</Button>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
