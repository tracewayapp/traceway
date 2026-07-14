<script lang="ts">
	import type { Snippet } from 'svelte';
	import { Input } from '$lib/components/ui/input';
	import { Button } from '$lib/components/ui/button';
	import * as Select from '$lib/components/ui/select';

	type TypeOption = {
		value: string;
		label: string;
	};

	type Props = {
		placeholder?: string;
		value?: string;
		typeValue?: string;
		typeOptions?: TypeOption[];
		onSearch: () => void;
		disabled?: boolean;
		/**
		 * Optional slot rendered between the type select and the Go button.
		 * Consumers can drop additional selects in here (e.g. a severity filter)
		 * so they visually belong to the same compound control. Your content is
		 * responsible for matching the 9-unit height and using rounded-none +
		 * border-r-0 to keep the joined look.
		 */
		children?: Snippet;
	};

	let {
		placeholder = 'Search...',
		value = $bindable(''),
		typeValue = $bindable(''),
		typeOptions = [],
		onSearch,
		disabled = false,
		children
	}: Props = $props();

	const typeLabel = $derived(typeOptions.find((o) => o.value === typeValue)?.label ?? '');
</script>

<div class="flex">
	<Input
		{placeholder}
		class="h-9 w-[250px] rounded-r-none border-r-0 shadow-none focus-visible:border-r focus-visible:border-ring focus-visible:ring-0 lg:w-[320px]"
		bind:value
		onkeydown={(e) => {
			if (e.key === 'Enter') onSearch();
		}}
	/>

	{#if typeOptions.length > 0}
		<Select.Root type="single" bind:value={typeValue}>
			<Select.Trigger class="h-9 w-[110px] rounded-none border-r-0 shadow-none">
				{typeLabel}
			</Select.Trigger>
			<Select.Content>
				{#each typeOptions as option}
					<Select.Item value={option.value} label={option.label}>
						{option.label}
					</Select.Item>
				{/each}
			</Select.Content>
		</Select.Root>
	{/if}

	{@render children?.()}

	<Button variant="outline" class="h-9 rounded-l-none shadow-none" onclick={onSearch} {disabled}>
		Go
	</Button>
</div>
