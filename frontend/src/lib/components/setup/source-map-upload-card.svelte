<script lang="ts">
	import {
		Card,
		CardContent,
		CardDescription,
		CardHeader,
		CardTitle
	} from '$lib/components/ui/card';
	import { KeyRound } from '@lucide/svelte';
	import { projectsState, isProjectReadonly } from '$lib/state/projects.svelte';
	import SourceMapSetup from '$lib/components/source-map-setup.svelte';

	let projectWithToken = $derived(projectsState.currentProject);

	// Flutter, iOS and Android upload debug symbols (symbols, dSYMs, R8 mappings); browsers upload source maps.
	const uploadsSymbols = $derived(
		projectWithToken?.framework === 'flutter' ||
			projectWithToken?.framework === 'ios' ||
			projectWithToken?.framework === 'android'
	);

	const isReadonly = $derived(isProjectReadonly(projectsState.currentProject));
</script>

{#if projectWithToken && !isReadonly}
	<Card>
		<CardHeader>
			<CardTitle class="flex items-center gap-2">
				<KeyRound class="h-5 w-5" />
				{uploadsSymbols ? 'Symbol Upload' : 'Source Map Upload'}
			</CardTitle>
			{#if !uploadsSymbols}
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
