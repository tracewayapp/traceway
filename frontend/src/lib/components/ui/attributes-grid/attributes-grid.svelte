<script lang="ts">
	import { ChevronDown, ChevronUp } from '@lucide/svelte';
	import AttributesView from './attributes-view.svelte';
	import type { FilterOperator } from '$lib/utils/attribute-filters';

	let {
		attributes,
		sorted = true,
		collapsedCount = 3,
		filterOperatorFor,
		onFilterOperator,
		expanded = $bindable(false)
	}: {
		attributes: Record<string, string>;
		sorted?: boolean;
		collapsedCount?: number;
		filterOperatorFor?: (key: string, value: string) => FilterOperator | null;
		// null clears the filter on that key and value.
		onFilterOperator?: (key: string, value: string, operator: FilterOperator | null) => void;
		expanded?: boolean;
	} = $props();

	const entries = $derived.by(() => {
		const items = Object.entries(attributes);
		return sorted ? items.sort((a, b) => a[0].localeCompare(b[0])) : items;
	});

	// The toggle takes a cell of its own, so hiding a single attribute behind it
	// would save no space.
	const collapsible = $derived(entries.length > collapsedCount + 1);
	const visibleEntries = $derived(
		expanded || !collapsible ? entries : entries.slice(0, collapsedCount)
	);
	const hiddenCount = $derived(entries.length - collapsedCount);
</script>

<div class="grid grid-cols-1 gap-3 sm:grid-cols-2 md:grid-cols-3">
	{#each visibleEntries as [key, value], __index (__index)}
		<AttributesView
			title={key}
			{value}
			activeOperator={filterOperatorFor?.(key, value) ?? null}
			onFilterOperator={onFilterOperator
				? (operator) => onFilterOperator(key, value, operator)
				: undefined}
		/>
	{/each}
	{#if collapsible}
		<button
			type="button"
			aria-expanded={expanded}
			onclick={() => (expanded = !expanded)}
			class="flex min-h-16 items-center justify-center gap-1.5 rounded-md border border-dashed p-3 text-xs font-medium text-muted-foreground transition-colors hover:border-foreground/40 hover:bg-muted/40 hover:text-foreground"
		>
			{#if expanded}
				<ChevronUp class="h-3.5 w-3.5" />
				Show less
			{:else}
				<ChevronDown class="h-3.5 w-3.5" />
				Show {hiddenCount} more {hiddenCount === 1 ? 'attribute' : 'attributes'}
			{/if}
		</button>
	{/if}
</div>
