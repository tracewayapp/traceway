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
		{ value: 'get', label: 'GET' },
		{ value: 'post', label: 'POST' },
		{ value: 'put', label: 'PUT' },
		{ value: 'patch', label: 'PATCH' },
		{ value: 'delete', label: 'DELETE' },
		{ value: 'options', label: 'OPTIONS' },
		{ value: 'head', label: 'HEAD' }
	];

	// A method selection and a root selection are mutually exclusive - picking one clears the other
	const selected = $derived(methodValue !== 'all' ? methodValue : rootValue);
	const label = $derived(
		methodOptions.find((option) => option.value === selected)?.label ??
			rootOptions.find((option) => option.value === selected)?.label ??
			'All'
	);

	function onValueChange(value: string | undefined) {
		if (!value) return;
		if (methodOptions.some((option) => option.value === value)) {
			methodValue = value;
			rootValue = 'all';
		} else {
			rootValue = value;
			methodValue = 'all';
		}
	}
</script>

<Select.Root type="single" value={selected} {onValueChange}>
	<Select.Trigger class="h-9 w-[120px] shrink-0 rounded-none border-r-0 shadow-none">
		<span class={METHOD_COLORS[label] ?? ''}>
			{label}
		</span>
	</Select.Trigger>
	<Select.Content>
		{#each rootOptions as option (option.value)}
			<Select.Item value={option.value} label={option.label}>{option.label}</Select.Item>
		{/each}
		<Select.Separator />
		{#each methodOptions as option (option.value)}
			<Select.Item value={option.value} label={option.label} class="font-mono text-sm">
				<span class={METHOD_COLORS[option.label] ?? ''}>{option.label}</span>
			</Select.Item>
		{/each}
	</Select.Content>
</Select.Root>
