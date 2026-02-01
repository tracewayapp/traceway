<script lang="ts">
	import ContextView from './context-view.svelte';

	let {
		attributes,
		sorted = true
	}: {
		attributes: Record<string, string>;
		sorted?: boolean;
	} = $props();

	const entries = $derived(() => {
		const items = Object.entries(attributes);
		if (sorted) {
			return items.sort((a, b) => a[0].localeCompare(b[0]));
		}
		return items;
	});
</script>

<div class="grid grid-cols-1 gap-3 sm:grid-cols-2 md:grid-cols-3">
	{#each entries() as [key, value]}
		<ContextView title={key} {value} />
	{/each}
</div>
