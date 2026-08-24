<script lang="ts">
	import * as Select from '$lib/components/ui/select';

	type Props = {
		rootValue?: string;
		methodValue?: string;
	};

	let { rootValue = $bindable('all'), methodValue = $bindable('all') }: Props = $props();

	const METHOD_COLORS: Record<string, string> = {
		GET: 'text-sky-600 dark:text-sky-400',
		POST: 'text-emerald-600 dark:text-emerald-400',
		PUT: 'text-amber-600 dark:text-amber-400',
		PATCH: 'text-orange-600 dark:text-orange-400',
		DELETE: 'text-rose-600 dark:text-rose-400',
		HEAD: 'text-violet-600 dark:text-violet-400',
		OPTIONS: 'text-violet-600 dark:text-violet-400'
	};

	const rootOptions = [
		{ value: 'all', label: 'All' },
		{ value: 'root', label: 'Root' },
		{ value: 'non_root', label: 'Non-root' }
	];

	const methodOptions = [
		{ value: 'all', label: 'Endpoints' },
		{ value: 'get', label: 'GET' },
		{ value: 'post', label: 'POST' },
		{ value: 'put', label: 'PUT' },
		{ value: 'patch', label: 'PATCH' },
		{ value: 'delete', label: 'DELETE' },
		{ value: 'options', label: 'OPTIONS' },
		{ value: 'head', label: 'HEAD' }
	];

	const rootLabel = $derived(
		rootOptions.find((option) => option.value === rootValue)?.label ?? 'All'
	);
	const methodLabel = $derived(
		methodOptions.find((option) => option.value === methodValue)?.label ?? 'Endpoints'
	);
</script>

<Select.Root type="single" bind:value={methodValue}>
	<Select.Trigger class="h-9 w-[120px] shrink-0 shadow-none sm:rounded-none sm:border-r-0">
		<span class={METHOD_COLORS[methodLabel] ?? ''}>
			{methodLabel}
		</span>
	</Select.Trigger>
	<Select.Content>
		{#each methodOptions as option (option.value)}
			<Select.Item
				value={option.value}
				label={option.label}
				class={option.value === 'all' ? '' : 'font-mono text-sm'}
			>
				<span class={METHOD_COLORS[option.label] ?? ''}>{option.label}</span>
			</Select.Item>
		{/each}
	</Select.Content>
</Select.Root>

<Select.Root type="single" bind:value={rootValue}>
	<Select.Trigger class="h-9 w-[110px] shrink-0 shadow-none sm:rounded-none sm:border-r-0">
		{rootLabel}
	</Select.Trigger>
	<Select.Content>
		{#each rootOptions as option (option.value)}
			<Select.Item value={option.value} label={option.label}>{option.label}</Select.Item>
		{/each}
	</Select.Content>
</Select.Root>
