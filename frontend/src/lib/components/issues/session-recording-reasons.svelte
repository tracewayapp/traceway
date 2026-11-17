<script lang="ts">
	import { resolve } from '$app/paths';
	import { projectsState } from '$lib/state/projects.svelte';
	let {
		reasons = []
	}: { reasons?: Array<{ triggerId: string; name: string; sequence: string; revision: string }> } =
		$props();
</script>

<div class="flex flex-wrap gap-2" aria-label="Recording triggers">
	{#each reasons as reason (`${reason.triggerId}:${reason.revision}`)}
		<a
			class="max-w-full rounded-md border border-primary/20 bg-primary/5 px-2 py-1 text-xs hover:bg-primary/10"
			href={resolve(
				`/sessions?projectId=${encodeURIComponent(projectsState.currentProjectId ?? '')}&triggerId=${encodeURIComponent(reason.triggerId)}&preset=24h`
			)}
			onclick={(event) => event.stopPropagation()}
			title={`Filter sessions captured by ${reason.name}`}
		>
			<span class="block font-medium">{reason.name}</span><span
				class="block font-mono break-all text-muted-foreground">{reason.sequence}</span
			>
		</a>
	{/each}
</div>
