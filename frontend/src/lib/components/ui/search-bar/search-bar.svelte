<script lang="ts">
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
	};

	let {
		placeholder = 'Search...',
		value = $bindable(''),
		typeValue = $bindable(''),
		typeOptions = [],
		onSearch,
		disabled = false
	}: Props = $props();

	const typeLabel = $derived(typeOptions.find((o) => o.value === typeValue)?.label ?? '');
</script>

<div class="-mt-2 flex">
	<Input
		{placeholder}
		class="h-9 w-[250px] rounded-r-none border-r-0 shadow-none focus-visible:border-r focus-visible:border-sidebar-accent focus-visible:ring-0 lg:w-[320px]"
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

	<Button variant="outline" class="h-9 rounded-l-none shadow-none" onclick={onSearch} {disabled}>
		Go
	</Button>
</div>
