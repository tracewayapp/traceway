<script lang="ts">
	import { Check, Plus } from '@lucide/svelte';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import * as Select from '$lib/components/ui/select';
	import {
		filterOperator,
		isExistenceOperator,
		operatorOption,
		type AttributeFilter,
		type FilterOperator
	} from '$lib/utils/attribute-filters';
	import type { AttributeConfig } from './fields';

	let {
		config,
		filters,
		editIndex = null,
		seed = null,
		onSubmit,
		onCancel
	}: {
		config: AttributeConfig;
		// The staged list the new filter joins.
		filters: AttributeFilter[];
		editIndex?: number | null;
		// A new filter that starts filled in, from a value clicked in the results.
		seed?: AttributeFilter | null;
		onSubmit: (filter: AttributeFilter, editIndex: number | null) => void;
		onCancel?: () => void;
	} = $props();

	const uid = $props.id();
	const start = $derived(editIndex !== null ? filters[editIndex] : seed);
	let key = $derived(start?.key ?? '');
	let value = $derived(start?.value ?? '');
	let operator = $derived<FilterOperator>(start ? filterOperator(start) : '=');
	let error = $state('');
	let valueSuggestions = $state<string[]>([]);

	const needsValue = $derived(!isExistenceOperator(operator));
	const selected = $derived(operatorOption(operator));

	// Values for the typed key, once it is one of the suggested keys.
	$effect(() => {
		const k = key.trim();
		const load = config.loadValues;
		if (!load || !k || (config.keys && !config.keys.includes(k))) {
			valueSuggestions = [];
			return;
		}
		let current = true;
		load(k)
			.then((values) => {
				if (current) valueSuggestions = values;
			})
			.catch(() => {
				if (current) valueSuggestions = [];
			});
		return () => {
			current = false;
		};
	});

	function submit() {
		const parsed = config.parse(key, value, operator, filters, editIndex);
		if (typeof parsed === 'string') {
			error = parsed;
			return;
		}
		error = '';
		onSubmit(parsed, editIndex);
		key = '';
		value = '';
		operator = '=';
	}

	function onKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter') {
			e.preventDefault();
			e.stopPropagation();
			submit();
		}
	}
</script>

<div class="flex flex-col gap-2">
	<div class="grid grid-cols-[1fr_auto] gap-2">
		<Input
			placeholder={config.keyPlaceholder ?? 'attribute.key'}
			aria-label="Attribute key"
			class="h-8 font-mono text-xs"
			list={config.keys?.length ? `${uid}-keys` : undefined}
			autocomplete="off"
			bind:value={key}
			onkeydown={onKeydown}
		/>
		{#if config.operators.length > 1}
			<Select.Root
				type="single"
				value={operator}
				onValueChange={(v) => (operator = v as FilterOperator)}
			>
				<Select.Trigger class="h-8 min-w-[120px] text-xs" aria-label="Operator">
					<span>
						<span class="font-mono text-muted-foreground">{selected.symbol}</span>
						{selected.label}
					</span>
				</Select.Trigger>
				<Select.Content>
					{#each config.operators as opt (opt.value)}
						<Select.Item value={opt.value} label={opt.label}>
							<span class="font-mono text-muted-foreground">{opt.symbol}</span>
							{opt.label}
						</Select.Item>
					{/each}
				</Select.Content>
			</Select.Root>
		{:else}
			<span class="self-center px-2 font-mono text-xs text-muted-foreground">=</span>
		{/if}
	</div>
	{#if needsValue}
		<Input
			placeholder={config.valuePlaceholder ?? 'Value'}
			aria-label="Attribute value"
			class="h-8 font-mono text-xs"
			list={valueSuggestions.length ? `${uid}-values` : undefined}
			autocomplete="off"
			bind:value
			onkeydown={onKeydown}
		/>
	{/if}
	{#if config.keys?.length}
		<datalist id="{uid}-keys">
			{#each config.keys as k (k)}<option value={k}></option>{/each}
		</datalist>
	{/if}
	{#if valueSuggestions.length}
		<datalist id="{uid}-values">
			{#each valueSuggestions as v (v)}<option value={v}></option>{/each}
		</datalist>
	{/if}
	{#if error}
		<p class="text-xs text-destructive" role="alert">{error}</p>
	{/if}
	<div class="flex justify-end gap-2">
		{#if onCancel}
			<Button variant="ghost" size="sm" class="h-7 px-2 text-xs" onclick={onCancel}>Cancel</Button>
		{/if}
		<Button variant="outline" size="sm" class="h-7 px-2 text-xs" onclick={submit}>
			{#if editIndex !== null}
				<Check class="h-3.5 w-3.5" /> Update filter
			{:else}
				<Plus class="h-3.5 w-3.5" /> Add filter
			{/if}
		</Button>
	</div>
</div>
