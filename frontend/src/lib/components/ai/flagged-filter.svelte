<script lang="ts">
	import * as Select from '$lib/components/ui/select';

	type Props = {
		value?: string;
		onChange?: (value: string) => void;
	};

	let { value = $bindable('all'), onChange }: Props = $props();

	const options = [
		{ value: 'all', label: 'All' },
		{ value: 'flagged', label: 'Flagged only' }
	];

	const label = $derived(options.find((o) => o.value === value)?.label ?? 'All');
</script>

<Select.Root type="single" bind:value onValueChange={(v) => onChange?.(v ?? 'all')}>
	<Select.Trigger class="h-9 w-[130px] rounded-none border-r-0 shadow-none">
		{label}
	</Select.Trigger>
	<Select.Content>
		{#each options as option}
			<Select.Item value={option.value} label={option.label}>
				{option.label}
			</Select.Item>
		{/each}
	</Select.Content>
</Select.Root>
