<script lang="ts">
	import { X } from '@lucide/svelte';
	import type { AttributeFilter } from '$lib/utils/attribute-filters';
	import { attributePill } from './fields';

	let {
		filter,
		onEdit,
		onRemove,
		pending = false,
		removeDisabled = false
	}: {
		filter: AttributeFilter;
		onEdit: () => void;
		onRemove: () => void;
		// In the drawer: added or changed, not applied yet.
		pending?: boolean;
		// On the page, where × runs the search: off while a request is out.
		removeDisabled?: boolean;
	} = $props();
</script>

<span
	class="inline-flex h-6 max-w-full items-center gap-1 rounded-full px-2 font-mono text-xs {pending
		? 'bg-primary/10 ring-1 ring-primary/40 ring-inset'
		: 'bg-muted'}"
>
	<button
		type="button"
		class="inline-flex h-6 min-w-0 items-center gap-1 hover:text-primary"
		title="Edit filter"
		aria-label="Edit filter {filter.key} {attributePill(filter)}{filter.exists
			? ''
			: ` ${filter.value || '""'}`}"
		data-filter-pill
		onclick={onEdit}
	>
		<span class="truncate text-muted-foreground" title={filter.key}>{filter.key}</span>
		<span class={filter.exclude ? 'text-red-500' : ''}>{attributePill(filter)}</span>
		{#if !filter.exists}
			<!-- Stored-value pages match an empty value too, which would otherwise read as nothing. -->
			<span class="truncate" title={filter.value}>{filter.value || '""'}</span>
		{/if}
	</button>
	<button
		type="button"
		aria-label="Remove filter {filter.key}"
		class="-mr-1.5 inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-full text-muted-foreground hover:text-foreground disabled:pointer-events-none disabled:opacity-50"
		disabled={removeDisabled}
		onclick={onRemove}
	>
		<X class="h-3 w-3" />
	</button>
</span>
