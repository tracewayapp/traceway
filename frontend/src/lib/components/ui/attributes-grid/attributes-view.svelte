<script lang="ts" module>
	export type AttributeFilterState = 'none' | 'include' | 'exclude';
</script>

<script lang="ts">
	import JSONTree from '@sveltejs/svelte-json-tree';
	import { Filter, FilterX } from '@lucide/svelte';
	import CopyButton from '$lib/components/traceway/copy-button.svelte';

	let {
		title,
		value,
		filterState = 'none',
		onFilterToggle
	}: {
		title: string;
		value: string;
		filterState?: AttributeFilterState;
		onFilterToggle?: () => void;
	} = $props();

	// Hidden-until-hover only where hovering exists: always visible on small
	// screens and coarse (touch) pointers, hover-revealed on md+ fine pointers.
	const hoverRevealClass =
		'opacity-100 md:opacity-0 md:group-hover:opacity-100 md:focus-visible:opacity-100 md:pointer-coarse:opacity-100';

	const filterTitle = $derived(
		filterState === 'none'
			? 'Filter by this value'
			: filterState === 'include'
				? 'Filtering by this value — click to clear'
				: 'Excluding this value — click to clear'
	);

	let showFull = $state(false);
	let clamped = $state(false);
	let valueEl = $state<HTMLElement | null>(null);

	$effect(() => {
		if (valueEl && !showFull) {
			clamped = valueEl.scrollHeight > valueEl.clientHeight + 1;
		}
	});

	const parsedJson = $derived(() => {
		try {
			const parsed = JSON.parse(value);
			if (typeof parsed === 'object' && parsed !== null) {
				return parsed;
			}
		} catch {
			// Not valid JSON
		}
		return null;
	});
</script>

<div class="group relative flex flex-col gap-1 rounded-md bg-muted p-3">
	<div class="flex items-center justify-between">
		<span class="text-xs font-medium text-muted-foreground">{title}</span>
		<div class="flex items-center gap-2">
			{#if onFilterToggle}
				<button
					onclick={onFilterToggle}
					class="transition-opacity {filterState === 'none'
						? `${hoverRevealClass} text-muted-foreground hover:text-foreground`
						: filterState === 'include'
							? 'text-blue-500 dark:text-blue-400'
							: 'text-red-500'}"
					title={filterTitle}
				>
					{#if filterState === 'exclude'}
						<FilterX class="h-3.5 w-3.5" />
					{:else}
						<Filter class="h-3.5 w-3.5 {filterState === 'include' ? 'fill-current' : ''}" />
					{/if}
				</button>
			{/if}
			<CopyButton
				bare
				text={value}
				iconClass="h-3.5 w-3.5"
				class="{hoverRevealClass} p-0 transition-opacity hover:bg-transparent"
			/>
		</div>
	</div>
	{#if parsedJson()}
		<div class="json-tree-container overflow-x-auto whitespace-normal">
			<JSONTree value={parsedJson()} />
		</div>
	{:else}
		<span
			bind:this={valueEl}
			class="font-mono text-sm break-all whitespace-pre-wrap {showFull ? '' : 'line-clamp-4'}"
			>{value}</span
		>
		{#if clamped || showFull}
			<button
				onclick={() => (showFull = !showFull)}
				class="self-start text-xs text-muted-foreground transition-colors hover:text-foreground"
			>
				{showFull ? 'Show less' : 'Show more...'}
			</button>
		{/if}
	{/if}
</div>

<style>
	.json-tree-container {
		--json-tree-font-size: 0.875rem;
		--json-tree-font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
		--json-tree-li-indentation: 1.25em;
		--json-tree-li-line-height: 1.5;

		/* Light mode colors */
		--json-tree-string-color: oklch(0.6 0.15 25);
		--json-tree-symbol-color: oklch(0.6 0.15 25);
		--json-tree-boolean-color: oklch(0.5 0.15 260);
		--json-tree-function-color: oklch(0.5 0.15 260);
		--json-tree-number-color: oklch(0.5 0.18 280);
		--json-tree-label-color: oklch(0.5 0.15 320);
		--json-tree-property-color: oklch(0.35 0.05 265);
		--json-tree-arrow-color: oklch(0.55 0.03 265);
		--json-tree-operator-color: oklch(0.55 0.03 265);
		--json-tree-null-color: oklch(0.55 0.03 265);
		--json-tree-undefined-color: oklch(0.55 0.03 265);
		--json-tree-date-color: oklch(0.55 0.03 265);
		--json-tree-internal-color: oklch(0.55 0.03 265);
		--json-tree-regex-color: oklch(0.6 0.15 25);
	}

	:global(.dark) .json-tree-container {
		/* Dark mode colors */
		--json-tree-string-color: oklch(0.75 0.15 25);
		--json-tree-symbol-color: oklch(0.75 0.15 25);
		--json-tree-boolean-color: oklch(0.7 0.15 260);
		--json-tree-function-color: oklch(0.7 0.15 260);
		--json-tree-number-color: oklch(0.7 0.18 280);
		--json-tree-label-color: oklch(0.7 0.15 320);
		--json-tree-property-color: oklch(0.9 0.02 265);
		--json-tree-arrow-color: oklch(0.65 0 0);
		--json-tree-operator-color: oklch(0.65 0 0);
		--json-tree-null-color: oklch(0.65 0 0);
		--json-tree-undefined-color: oklch(0.65 0 0);
		--json-tree-date-color: oklch(0.65 0 0);
		--json-tree-internal-color: oklch(0.65 0 0);
		--json-tree-regex-color: oklch(0.75 0.15 25);
	}
</style>
