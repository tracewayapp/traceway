<script lang="ts">
	import {
		Card,
		CardContent,
		CardDescription,
		CardHeader,
		CardTitle
	} from '$lib/components/ui/card';
	import { KeyRound } from 'lucide-svelte';
	import { projectsState } from '$lib/state/projects.svelte';
	import { authState } from '$lib/state/auth.svelte';
	import SourceMapSetup from '$lib/components/source-map-setup.svelte';

	let projectWithToken = $derived(projectsState.currentProject);

	const isFlutter = $derived(projectWithToken?.framework === 'flutter');

	const isReadonly = $derived(
		authState.getRoleForOrganization(projectsState.currentProject?.organizationId ?? 0) ===
			'readonly'
	);
</script>

{#if projectWithToken && !isReadonly}
	<Card>
		<CardHeader>
			<CardTitle class="flex items-center gap-2">
				<KeyRound class="h-5 w-5" />
				{isFlutter ? 'Symbol Upload' : 'Source Map Upload'}
			</CardTitle>
			{#if !isFlutter}
				<CardDescription>
					Upload source maps to see original file names and line numbers in stack traces from
					minified code.
				</CardDescription>
			{/if}
		</CardHeader>
		<CardContent>
			<SourceMapSetup />
		</CardContent>
	</Card>
{/if}
