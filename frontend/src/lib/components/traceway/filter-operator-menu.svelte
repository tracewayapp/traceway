<script lang="ts">
	import type { Snippet } from 'svelte';
	import { Check, X } from '@lucide/svelte';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import { operatorOptions, type FilterOperator } from '$lib/utils/attribute-filters';

	let {
		active,
		label,
		onSelect,
		onClear,
		triggerClass,
		triggerTitle,
		triggerAriaLabel,
		children
	}: {
		active: FilterOperator | null;
		label?: string;
		onSelect: (operator: FilterOperator) => void;
		onClear?: () => void;
		triggerClass?: string;
		triggerTitle?: string;
		triggerAriaLabel?: string;
		children: Snippet;
	} = $props();
</script>

<DropdownMenu.Root>
	<DropdownMenu.Trigger class={triggerClass} title={triggerTitle} aria-label={triggerAriaLabel}>
		{@render children()}
	</DropdownMenu.Trigger>
	<DropdownMenu.Content align="end" class="min-w-44">
		{#if label}
			<DropdownMenu.Label
				class="max-w-64 truncate font-mono text-xs font-normal text-muted-foreground"
				title={label}
			>
				{label}
			</DropdownMenu.Label>
		{/if}
		{#each operatorOptions as option (option.value)}
			<DropdownMenu.Item onclick={() => onSelect(option.value)}>
				<Check class="h-4 w-4 {active === option.value ? '' : 'opacity-0'}" />
				<span class="w-4 font-mono text-muted-foreground">{option.symbol}</span>
				{option.label}
			</DropdownMenu.Item>
		{/each}
		{#if active && onClear}
			<DropdownMenu.Separator />
			<DropdownMenu.Item variant="destructive" onclick={onClear}>
				<X class="h-4 w-4" />
				Clear filter
			</DropdownMenu.Item>
		{/if}
	</DropdownMenu.Content>
</DropdownMenu.Root>
