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
		 * Optional slot for additional filters (extra selects, toggles). They render
		 * as standalone controls after the joined search unit and wrap onto their
		 * own row on narrow screens instead of squeezing the input.
		 */
		children?: Snippet;
		/**
		 * Optional slot rendered inside the joined search unit, between the type
		 * select (if any) and the Go button. Use for a control that should share
		 * the pill's borders instead of standing on its own - style it with
		 * `rounded-none border-r-0` to match.
		 */
		pillEnd?: Snippet;
	};

	let {
		placeholder = 'Search...',
		value = $bindable(''),
		typeValue = $bindable(''),
		typeOptions = [],
		onSearch,
		disabled = false,
		children,
		pillEnd
	}: Props = $props();

	const typeLabel = $derived(typeOptions.find((o) => o.value === typeValue)?.label ?? '');
</script>

<div class="flex flex-wrap items-center gap-2">
	<div
		class="flex w-full flex-wrap items-center gap-2 sm:w-auto sm:min-w-0 sm:flex-initial sm:flex-nowrap sm:gap-0"
	>
		<Input
			{placeholder}
			class="h-9 w-full min-w-[140px] shadow-none sm:w-[250px] sm:rounded-r-none sm:border-r-0 sm:focus-visible:border-r sm:focus-visible:border-ring sm:focus-visible:ring-0 lg:w-[320px]"
			bind:value
			onkeydown={(e) => {
				if (e.key === 'Enter') onSearch();
			}}
		/>

		{#if typeOptions.length > 0}
			<Select.Root type="single" bind:value={typeValue}>
				<Select.Trigger
					class="h-9 w-fit shrink-0 shadow-none sm:rounded-none sm:border-r-0"
				>
					{typeLabel}
				</Select.Trigger>
				<Select.Content>
					{#each typeOptions as option (option.value)}
						<Select.Item value={option.value} label={option.label}>
							{option.label}
						</Select.Item>
					{/each}
				</Select.Content>
			</Select.Root>
		{/if}

		{@render pillEnd?.()}

		<Button
			variant="outline"
			class="h-9 shadow-none sm:rounded-l-none"
			onclick={onSearch}
			{disabled}
		>
			Go
		</Button>
	</div>

	{@render children?.()}
</div>
