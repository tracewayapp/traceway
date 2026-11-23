<script lang="ts">
	import type { Snippet } from 'svelte';
	import { Check, X } from '@lucide/svelte';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import {
		operatorOptions,
		type FilterOperator,
		type OperatorOption
	} from '$lib/utils/attribute-filters';

	let {
		active,
		label,
		options = operatorOptions,
		onSelect,
		onClear,
		triggerClass,
		triggerTitle,
		triggerAriaLabel,
		children,
		extra
	}: {
		active: FilterOperator | null;
		label?: string;
		options?: readonly OperatorOption[];
		onSelect: (operator: FilterOperator) => void;
		onClear?: () => void;
		triggerClass?: string;
		triggerTitle?: string;
		triggerAriaLabel?: string;
		children: Snippet;
		// More actions on the same value, below the operators.
		extra?: Snippet;
	} = $props();
</script>

<DropdownMenu.Root>
	<DropdownMenu.Trigger class={triggerClass} title={triggerTitle} aria-label={triggerAriaLabel}>
		{@render children()}
	</DropdownMenu.Trigger>
	<DropdownMenu.Content align="end" class="min-w-48">
		{#if label}
			<DropdownMenu.Label
				class="max-w-64 truncate font-mono text-xs font-normal text-muted-foreground"
				title={label}
			>
				{label}
			</DropdownMenu.Label>
		{/if}
		{#each options as option, i (option.value)}
			{#if i > 0 && option.value === '?='}
				<DropdownMenu.Separator />
			{/if}
			<DropdownMenu.Item onclick={() => onSelect(option.value)}>
				<Check class="h-4 w-4 {active === option.value ? '' : 'opacity-0'}" />
				<span class="w-4 font-mono text-muted-foreground">{option.symbol}</span>
				{option.label}
			</DropdownMenu.Item>
		{/each}
		{#if extra}
			<DropdownMenu.Separator />
			{@render extra()}
		{/if}
		{#if active && onClear}
			<DropdownMenu.Separator />
			<DropdownMenu.Item variant="destructive" onclick={onClear}>
				<X class="h-4 w-4" />
				Clear filter
			</DropdownMenu.Item>
		{/if}
	</DropdownMenu.Content>
</DropdownMenu.Root>
