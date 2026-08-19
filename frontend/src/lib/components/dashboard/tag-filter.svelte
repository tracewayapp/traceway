<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Select from '$lib/components/ui/select';
	import { X } from '@lucide/svelte';

	let {
		tagKeys = [],
		activeFilters = {},
		onFilterChange,
		onLoadTagValues
	} = $props<{
		tagKeys: string[];
		activeFilters: Record<string, string>;
		onFilterChange: (filters: Record<string, string>) => void;
		onLoadTagValues: (key: string) => Promise<string[]>;
	}>();

	let selectedKey = $state('');
	let tagValueOptions = $state<Record<string, string[]>>({});
	let loadingValues = $state(false);

	async function handleKeySelect(key: string) {
		selectedKey = key;
		if (!tagValueOptions[key]) {
			loadingValues = true;
			try {
				tagValueOptions[key] = await onLoadTagValues(key);
			} catch {
				tagValueOptions[key] = [];
			} finally {
				loadingValues = false;
			}
		}
	}

	function handleValueSelect(value: string) {
		if (!selectedKey) return;
		const newFilters = { ...activeFilters, [selectedKey]: value };
		onFilterChange(newFilters);
		selectedKey = '';
	}

	function removeFilter(key: string) {
		const newFilters = { ...activeFilters };
		delete newFilters[key];
		onFilterChange(newFilters);
	}

	const availableKeys = $derived(tagKeys.filter((k: string) => !(k in activeFilters)));
</script>

<div class="flex flex-wrap items-center gap-2">
	{#each Object.entries(activeFilters) as [key, value], __index (__index)}
		<div class="flex items-center gap-1 rounded-md border bg-muted/50 px-2 py-1 text-xs">
			<span class="font-medium">{key}</span>
			<span class="text-muted-foreground">=</span>
			<span>{value}</span>
			<button class="ml-1 rounded hover:bg-muted" onclick={() => removeFilter(key)}>
				<X class="h-3 w-3" />
			</button>
		</div>
	{/each}

	{#if availableKeys.length > 0}
		{#if selectedKey}
			<Select.Root
				type="single"
				onValueChange={(v) => {
					if (v) handleValueSelect(v);
				}}
			>
				<Select.Trigger class="h-7 w-[140px] text-xs">
					{#if loadingValues}
						Loading...
					{:else}
						Select value
					{/if}
				</Select.Trigger>
				<Select.Content>
					{#each tagValueOptions[selectedKey] ?? [] as val, __index (__index)}
						<Select.Item value={val}>{val}</Select.Item>
					{/each}
				</Select.Content>
			</Select.Root>
			<Button variant="ghost" size="sm" class="h-7 px-2 text-xs" onclick={() => (selectedKey = '')}>
				Cancel
			</Button>
		{:else}
			<Select.Root
				type="single"
				onValueChange={(v) => {
					if (v) handleKeySelect(v);
				}}
			>
				<Select.Trigger class="h-7 w-[140px] text-xs">+ Filter by tag</Select.Trigger>
				<Select.Content>
					{#each availableKeys as key, __index (__index)}
						<Select.Item value={key}>{key}</Select.Item>
					{/each}
				</Select.Content>
			</Select.Root>
		{/if}
	{/if}
</div>
