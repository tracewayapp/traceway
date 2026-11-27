<script lang="ts">
	import PageHeader from '$lib/components/traceway/page-header.svelte';
	import EmptyState from '$lib/components/traceway/empty-state.svelte';
	import DataCollection from '$lib/components/sdk-config/data-collection.svelte';
	import { projectsState, SDK_CONFIG_FRAMEWORKS } from '$lib/state/projects.svelte';
	import { Upload } from '@lucide/svelte';

	const project = $derived(projectsState.currentProject);
	const supported = $derived(!!project && SDK_CONFIG_FRAMEWORKS.includes(project.framework));
</script>

<div class="space-y-4">
	<PageHeader
		title="Data Collection"
		description="Decide remotely what the SDK sends, per group of clients."
	/>

	{#if supported}
		{#key project?.id}
			<div class="max-w-4xl">
				<DataCollection />
			</div>
		{/key}
	{:else if project}
		<EmptyState
			message="Remote collection settings are available for browser, Flutter, iOS and Android projects, whose SDKs ask Traceway what to send."
		>
			{#snippet icon()}
				<Upload class="h-6 w-6 text-muted-foreground" />
			{/snippet}
		</EmptyState>
	{/if}
</div>
