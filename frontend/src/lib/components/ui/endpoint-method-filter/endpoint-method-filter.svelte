<script lang="ts">
    import * as Select from '$lib/components/ui/select';

	type Props = {
		value?: string;
	};

	let { value = $bindable('all') }: Props = $props();

    const METHOD_COLORS: Record<string, string> = {
		GET: 'text-sky-600 dark:text-sky-400',
		POST: 'text-emerald-600 dark:text-emerald-400',
		PUT: 'text-amber-600 dark:text-amber-400',
		PATCH: 'text-orange-600 dark:text-orange-400',
		DELETE: 'text-rose-600 dark:text-rose-400',
		HEAD: 'text-violet-600 dark:text-violet-400',
		OPTIONS: 'text-violet-600 dark:text-violet-400'
	};

	const options = [
		{ value: 'all', label: 'Methods' },
		{ value: 'get', label: 'GET' },
		{ value: 'post', label: 'POST' },
		{ value: 'put', label: 'PUT' },
		{ value: 'patch', label: 'PATCH' },
		{ value: 'delete', label: 'DELETE' },
		{ value: 'options', label: 'OPTIONS' },
		{ value: 'head', label: 'HEAD' }
	];
    const label = $derived(options.find((option) => option.value === value)?.label ?? '');
</script>

<Select.Root type="single" bind:value={value}>
	<Select.Trigger class="h-9 w-[120px] shrink-0 shadow-none">
		<span class={METHOD_COLORS[label] ?? ''}>
            {label}
        </span>
	</Select.Trigger>
	<Select.Content>
		{#each options as option (option.value)}
			<Select.Item value={option.value} label={option.label} class="font-mono text-sm">
				<span class={METHOD_COLORS[option.label] ?? ''}>{option.label}</span>
			</Select.Item>
		{/each}
	</Select.Content>
</Select.Root>

