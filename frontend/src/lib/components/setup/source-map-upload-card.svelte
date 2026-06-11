<script lang="ts">
	import {
		Card,
		CardContent,
		CardDescription,
		CardHeader,
		CardTitle
	} from '$lib/components/ui/card';
	import { Button } from '$lib/components/ui/button';
	import { KeyRound } from 'lucide-svelte';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import bash from 'svelte-highlight/languages/bash';
	import { projectsState } from '$lib/state/projects.svelte';
	import { authState } from '$lib/state/auth.svelte';
	import CopyableInline from './copyable-inline.svelte';
	import CopyableCodeBlock from './copyable-code-block.svelte';

	let projectWithToken = $derived(projectsState.currentProject);
	let generatingToken = $state(false);

	const isReadonly = $derived(
		authState.getRoleForOrganization(projectsState.currentProject?.organizationId ?? 0) ===
			'readonly'
	);
	const sourceMapToken = $derived(projectWithToken?.sourceMapToken ?? null);

	const uploadCommand = $derived(
		projectWithToken && sourceMapToken
			? `npx @tracewayapp/sourcemap-upload --url ${projectWithToken.backendUrl} --token ${sourceMapToken} --version YOUR_VERSION --directory dist/assets`
			: ''
	);

	async function generateToken() {
		generatingToken = true;
		try {
			await projectsState.generateSourceMapToken();
		} finally {
			generatingToken = false;
		}
	}
</script>

{#if projectWithToken && !isReadonly}
	<Card>
		<CardHeader>
			<CardTitle class="flex items-center gap-2">
				<KeyRound class="h-5 w-5" />
				Source Map Upload
			</CardTitle>
			<CardDescription>
				Upload source maps to see original file names and line numbers in stack traces from
				minified code.
			</CardDescription>
		</CardHeader>
		<CardContent>
			{#if sourceMapToken}
				<div class="space-y-4">
					<div>
						<p class="mb-2 text-sm font-medium">Upload Token</p>
						<CopyableInline value={sourceMapToken} />
					</div>
					<div>
						<p class="mb-2 text-sm font-medium">Usage</p>
						<CopyableCodeBlock code={uploadCommand} language={bash} />
					</div>
				</div>
			{:else}
				<p class="mb-4 text-sm text-muted-foreground">
					Generate an upload token to start uploading source maps as part of your build process.
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
