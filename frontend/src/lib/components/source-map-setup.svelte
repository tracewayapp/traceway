<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { KeyRound, RefreshCw } from '@lucide/svelte';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import * as Tabs from '$lib/components/ui/tabs';
	import { toast } from 'svelte-sonner';
	import javascript from 'svelte-highlight/languages/javascript';
	import typescript from 'svelte-highlight/languages/typescript';
	import bash from 'svelte-highlight/languages/bash';
	import CopyButton from '$lib/components/traceway/copy-button.svelte';
	import CopyableCodeBlock from '$lib/components/setup/copyable-code-block.svelte';
	import { projectsState, isProjectReadonly } from '$lib/state/projects.svelte';

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

	const pluginInstallCommand = 'npm install -D @tracewayapp/bundler-plugin';

	const project = $derived(projectsState.currentProject);
	const sourceMapToken = $derived(project?.sourceMapToken ?? null);
	const isReadonly = $derived(isProjectReadonly(project));

	const isFlutter = $derived(project?.framework === 'flutter');
	const isIOS = $derived(project?.framework === 'ios');
	const isAndroid = $derived(project?.framework === 'android');
	const artifactLabel = $derived(isFlutter || isIOS || isAndroid ? 'debug symbols' : 'source maps');

	const showBundlerSetup = $derived(project?.framework !== 'react-native');

	const uploadCommand = $derived(
		project && sourceMapToken
			? `npx @tracewayapp/sourcemap-upload \\
  --url ${project.backendUrl} \\
  --token ${sourceMapToken} \\
  --directory ${showBundlerSetup ? bundlerConfigs[bundler].directory : 'dist'}`
			: ''
	);

	const flutterBuildCommand =
		'flutter build apk --release --obfuscate --split-debug-info=build/symbols';

	const flutterUploadCommand = $derived(
		project && sourceMapToken
			? `dart run traceway:upload_symbols \\
  --token ${sourceMapToken} \\
  --url ${project.backendUrl}`
			: ''
	);

	const iosBuildCommand =
		'xcodebuild -scheme MyApp -configuration Release \\\n  -archivePath build/MyApp.xcarchive archive';

	const iosUploadCommand = $derived(
		project && sourceMapToken
			? `curl -X POST ${project.backendUrl}/api/symbols/upload \\
  -H "Authorization: Bearer ${sourceMapToken}" \\
  -F "files=@build/MyApp.xcarchive/dSYMs/MyApp.app.dSYM/Contents/Resources/DWARF/MyApp"`
			: ''
	);

	const androidSetupSnippet = $derived(
		project && sourceMapToken
			? `plugins {
  id("com.tracewayapp.symbols")
}

android {
  buildTypes {
    release { isMinifyEnabled = true }
  }
}

traceway {
  token = "${sourceMapToken}"
  url = "${project.backendUrl}"
}`
			: ''
	);

	const androidUploadCommand = './gradlew assembleRelease uploadReleaseTracewaySymbols';

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
			toast.success('Successfully regenerated the Upload Token');
		} finally {
			generatingToken = false;
		}
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
				<CopyButton text={sourceMapToken ?? ''} />
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
		{#if isFlutter}
			<div>
				<p class="mb-2 text-sm font-medium">Step 1: Build with obfuscation enabled</p>
				<CopyableCodeBlock code={flutterBuildCommand} language={bash} />
				<p class="mt-2 text-xs text-muted-foreground">
					This writes a per-architecture .symbols file into build/symbols. The example builds an
					Android APK; other targets emit their own symbol files in the same directory.
				</p>
			</div>
			<div>
				<p class="mb-2 text-sm font-medium">Step 2: Upload the symbols after each release build</p>
				<CopyableCodeBlock code={flutterUploadCommand} language={bash} />
				<p class="mt-2 text-xs text-muted-foreground">
					Run from your project root after each release. The uploader auto-discovers build/symbols
					and pushes every architecture in one go; symbols are unique per build, so re-upload on
					each release. In CI, pass the token as <code class="font-mono">TRACEWAY_UPLOAD_TOKEN</code
					>
					instead of the flag.
				</p>
			</div>
		{:else if isIOS}
			<div>
				<p class="mb-2 text-sm font-medium">Step 1: Build an archive with dSYMs</p>
				<CopyableCodeBlock code={iosBuildCommand} language={bash} />
				<p class="mt-2 text-xs text-muted-foreground">
					Release builds emit a .dSYM bundle per architecture under the archive's dSYMs directory.
					Replace MyApp with your scheme name.
				</p>
			</div>
			<div>
				<p class="mb-2 text-sm font-medium">Step 2: Upload the dSYM after each release build</p>
				<CopyableCodeBlock code={iosUploadCommand} language={bash} />
				<p class="mt-2 text-xs text-muted-foreground">
					Upload the Mach-O DWARF inside the .dSYM bundle. Symbols are keyed by build UUID, so
					re-upload on each release.
				</p>
			</div>
		{:else if isAndroid}
			<div>
				<p class="mb-2 text-sm font-medium">Step 1: Apply the Traceway symbols Gradle plugin</p>
				<CopyableCodeBlock code={androidSetupSnippet} language={javascript} />
				<p class="mt-2 text-xs text-muted-foreground">
					Add to your app module's <code class="font-mono">build.gradle.kts</code>. The plugin
					embeds a ProGuard UUID into BuildConfig (matching Honeycomb's
					<code class="font-mono">app.debug.proguard_uuid</code>) and names the uploaded mapping
					<code class="font-mono">&lt;uuid&gt;.txt</code>.
				</p>
			</div>
			<div>
				<p class="mb-2 text-sm font-medium">Step 2: Build and upload after each release</p>
				<CopyableCodeBlock code={androidUploadCommand} language={bash} />
				<p class="mt-2 text-xs text-muted-foreground">
					Uploads the R8 <code class="font-mono">mapping.txt</code> and the unstripped native
					<code class="font-mono">.so</code> libraries. Native symbols are keyed by GNU build-id, so re-upload
					on each release.
				</p>
			</div>
		{:else}
			{#if showBundlerSetup}
				<div>
					<p class="mb-2 text-sm font-medium">Step 1: Install the bundler plugin</p>
					<CopyableCodeBlock code={pluginInstallCommand} language={bash} />
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
					<CopyableCodeBlock
						code={bundlerConfigs[bundler].code}
						language={bundlerConfigs[bundler].language}
					/>
				</div>
			{/if}
			<div>
				<p class="mb-2 text-sm font-medium">
					{showBundlerSetup ? 'Step 3: Upload after your production build' : 'Usage'}
				</p>
				<CopyableCodeBlock code={uploadCommand} language={bash} />
			</div>
		{/if}
	</div>
{:else if isReadonly}
	<p class="text-sm text-muted-foreground">
		An upload token is required to upload {artifactLabel}. Ask an organization admin to generate one
		from the Connection page.
	</p>
{:else}
	<div class="flex items-center justify-between gap-4">
		{#if isFlutter}
			<p class="text-sm text-muted-foreground">
				Plain release builds already report readable traces. Only obfuscated builds (<code
					class="rounded bg-muted px-1 py-0.5 font-mono text-xs">--obfuscate</code
				>) need this: generate a token, then upload your
				<code class="rounded bg-muted px-1 py-0.5 font-mono text-xs">.symbols</code>
				after each release to resolve their stack traces.
				<a
					href="https://docs.tracewayapp.com/client/flutter"
					target="_blank"
					rel="noopener noreferrer"
					class="underline hover:text-foreground">Flutter docs</a
				>
			</p>
		{:else if isIOS}
			<p class="text-sm text-muted-foreground">
				Release crashes report against stripped machine code. Generate a token, then upload your
				<code class="rounded bg-muted px-1 py-0.5 font-mono text-xs">.dSYM</code> after each release
				to resolve their stack traces.
				<a
					href="https://docs.tracewayapp.com/client/ios"
					target="_blank"
					rel="noopener noreferrer"
					class="underline hover:text-foreground">iOS docs</a
				>
			</p>
		{:else if isAndroid}
			<p class="text-sm text-muted-foreground">
				Release builds obfuscate Kotlin/Java with R8 and strip native code. Generate a token, then
				upload your <code class="rounded bg-muted px-1 py-0.5 font-mono text-xs">mapping.txt</code>
				and native <code class="rounded bg-muted px-1 py-0.5 font-mono text-xs">.so</code> libraries
				after each release to resolve their stack traces.
				<a
					href="https://docs.tracewayapp.com/symbolicator/android"
					target="_blank"
					rel="noopener noreferrer"
					class="underline hover:text-foreground">Android docs</a
				>
			</p>
		{:else}
			<p class="text-sm text-muted-foreground">
				Generate an upload token to start uploading {artifactLabel} as part of your build process.
			</p>
		{/if}
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
