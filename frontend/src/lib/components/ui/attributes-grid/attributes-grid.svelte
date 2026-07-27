<script lang="ts">
	import AttributesView, { type AttributeFilterState } from './attributes-view.svelte';

	let {
		attributes,
		sorted = true,
		collapsedCount = 3,
		filterStateFor,
		onFilterToggle
	}: {
		attributes: Record<string, string>;
		sorted?: boolean;
		collapsedCount?: number;
		filterStateFor?: (key: string, value: string) => AttributeFilterState;
		onFilterToggle?: (key: string, value: string) => void;
	} = $props();

	let expanded = $state(false);

	const entries = $derived(() => {
		const items = Object.entries(attributes);
		if (sorted) {
			return items.sort((a, b) => a[0].localeCompare(b[0]));
		}
		return items;
	});

	const visibleEntries = $derived(() => {
		const all = entries();
		if (expanded || all.length <= collapsedCount) return all;
		return all.slice(0, collapsedCount);
	});

	const hiddenCount = $derived(() => {
		const all = entries();
		if (expanded || all.length <= collapsedCount) return 0;
		return all.length - collapsedCount;
	});
</script>

<div>
	<div class="grid grid-cols-1 gap-3 sm:grid-cols-2 md:grid-cols-3">
		{#each visibleEntries() as [key, value]}
			<AttributesView
				title={key}
				{value}
				filterState={filterStateFor?.(key, value) ?? 'none'}
				onFilterToggle={onFilterToggle ? () => onFilterToggle(key, value) : undefined}
			/>
		{/each}
	</div>
	{#if hiddenCount() > 0}
		<div class="flex justify-end">
			<button
				onclick={() => expanded = true}
				class="mt-2 text-xs text-muted-foreground hover:text-foreground transition-colors"
			>
				Show {hiddenCount()} more...
			</button>
		</div>
	{:else if expanded && entries().length > collapsedCount}
		<div class="flex justify-end">
			<button
				onclick={() => expanded = false}
				class="mt-2 text-xs text-muted-foreground hover:text-foreground transition-colors"
			>
				Show less
			</button>
		</div>
	{/if}
</div>
