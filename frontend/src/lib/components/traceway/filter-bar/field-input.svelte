<script lang="ts">
	import { Check, ChevronDown } from '@lucide/svelte';
	import { Input } from '$lib/components/ui/input';
	import * as Command from '$lib/components/ui/command';
	import * as Popover from '$lib/components/ui/popover';
	import * as Select from '$lib/components/ui/select';
	import {
		SEARCHABLE_AFTER,
		anyLabelOf,
		clearedField,
		fieldError,
		fieldValueLabel,
		isFieldSet,
		labelInSentence,
		type FilterField,
		type FilterOption,
		type FilterValues
	} from './fields';

	let {
		field,
		values,
		onChange,
		onSubmit,
		id,
		highlight = false,
		options: loadedOptions,
		loading = false,
		failed = false,
		changed = false
	}: {
		field: FilterField;
		values: FilterValues;
		onChange: (values: FilterValues) => void;
		onSubmit: () => void;
		// The drawer focuses this field when it was opened from the field's pill.
		id: string;
		highlight?: boolean;
		// Options fetched by the drawer for a field with `loadOptions`.
		options?: readonly FilterOption[];
		loading?: boolean;
		failed?: boolean;
		// Differs from what is applied, so Apply would change it.
		changed?: boolean;
	} = $props();

	const set = $derived(isFieldSet(field, values));
	const error = $derived(fieldError(field, values));
	const empty = $derived(field.kind === 'select' ? (field.emptyValue ?? '') : '');
	const anyLabel = $derived(anyLabelOf(field));
	const current = $derived(field.kind === 'select' ? (values[field.key] ?? empty) : '');
	const options = $derived.by<FilterOption[]>(() => {
		if (field.kind !== 'select') return [];
		const list = (loadedOptions ?? field.options).filter((o) => o.value !== empty);
		// A value from a link or an earlier range stays selectable even when the data no longer has it.
		return set && !list.some((o) => o.value === current)
			? [{ value: current, label: current }, ...list]
			: list;
	});
	const searchable = $derived(
		field.kind === 'select' && (!!field.searchable || options.length > SEARCHABLE_AFTER)
	);
	const currentLabel = $derived(
		set
			? (options.find((o) => o.value === current)?.label ?? fieldValueLabel(field, values))
			: anyLabel
	);

	let comboOpen = $state(false);

	function pick(value: string) {
		onChange({ ...values, [field.key]: value });
		comboOpen = false;
	}

	function onKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter') {
			e.preventDefault();
			onSubmit();
		}
	}
</script>

<!-- The "Any" item always matches, so a list that loaded nothing says so here, not in Command.Empty. -->
{#snippet optionsState()}
	{#if loading}
		<p class="px-2 py-1.5 text-xs text-muted-foreground">Loading...</p>
	{:else if failed}
		<p class="px-2 py-1.5 text-xs text-destructive">
			Could not load the values. Reopen the filters to try again.
		</p>
	{:else if field.kind === 'select' && field.loadOptions && options.length === 0}
		<p class="px-2 py-1.5 text-xs text-muted-foreground">No values reported.</p>
	{/if}
{/snippet}

<div
	{id}
	class="space-y-1.5 rounded-md transition-shadow {highlight
		? 'ring-2 ring-primary/40 ring-offset-4 ring-offset-background'
		: ''}"
>
	<div class="flex items-center justify-between gap-2">
		<span class="flex items-center gap-1.5 text-xs font-medium" id="{id}-label">
			{field.label}
			{#if changed}
				<span class="h-1.5 w-1.5 rounded-full bg-primary" title="Changed, not applied yet"></span>
				<span class="sr-only">(changed)</span>
			{/if}
		</span>
		{#if set}
			<button
				type="button"
				class="text-xs text-muted-foreground hover:text-foreground"
				aria-label="Clear {field.label}"
				onclick={() => onChange(clearedField(field, values))}>Clear</button
			>
		{/if}
	</div>

	{#if field.kind === 'select' && searchable}
		<Popover.Root bind:open={comboOpen}>
			<Popover.Trigger
				class="flex h-9 w-full items-center justify-between gap-2 rounded-md border border-input bg-transparent px-3 text-sm shadow-xs outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 dark:bg-input/30 dark:hover:bg-input/50 {set
					? ''
					: 'text-muted-foreground'}"
				aria-labelledby="{id}-label"
				data-filter-input
			>
				<span class="truncate {field.mono && set ? 'font-mono text-xs' : ''}">{currentLabel}</span>
				<ChevronDown class="h-4 w-4 shrink-0 opacity-50" />
			</Popover.Trigger>
			<Popover.Content align="start" class="w-(--bits-popover-anchor-width) p-0">
				<Command.Root>
					<Command.Input placeholder="Find {labelInSentence(field.label)}..." />
					<Command.List class="max-h-72">
						<Command.Empty>Nothing matches.</Command.Empty>
						<Command.Group>
							<Command.Item value={anyLabel} onSelect={() => pick(empty)}>
								<Check class="h-4 w-4 {set ? 'opacity-0' : ''}" />
								<span class="text-muted-foreground">{anyLabel}</span>
							</Command.Item>
							{#each options as option (option.value)}
								<Command.Item value={option.label} onSelect={() => pick(option.value)}>
									<Check class="h-4 w-4 {current === option.value ? '' : 'opacity-0'}" />
									<span class="truncate {field.mono ? 'font-mono text-xs' : ''}"
										>{option.label}</span
									>
								</Command.Item>
							{/each}
						</Command.Group>
						{@render optionsState()}
					</Command.List>
				</Command.Root>
			</Popover.Content>
		</Popover.Root>
	{:else if field.kind === 'select'}
		<Select.Root
			type="single"
			value={current}
			onValueChange={(v) => onChange({ ...values, [field.key]: v ?? '' })}
		>
			<Select.Trigger
				class="h-9 w-full {set ? '' : 'text-muted-foreground'}"
				aria-labelledby="{id}-label"
				data-filter-input
			>
				<span
					class="truncate {field.mono && set ? 'font-mono text-xs' : ''} {set
						? (options.find((o) => o.value === current)?.class ?? '')
						: ''}">{currentLabel}</span
				>
			</Select.Trigger>
			<Select.Content>
				<Select.Item value={empty} label={anyLabel}>{anyLabel}</Select.Item>
				{#each options as option (option.value)}
					<Select.Item value={option.value} label={option.label}>
						<span class="{field.mono ? 'font-mono text-xs' : ''} {option.class ?? ''}"
							>{option.label}</span
						>
					</Select.Item>
				{/each}
				{@render optionsState()}
			</Select.Content>
		</Select.Root>
	{:else if field.kind === 'range'}
		<div class="flex items-center gap-2" role="group" aria-labelledby="{id}-label">
			<Input
				inputmode="decimal"
				placeholder="Min"
				aria-label="Minimum {labelInSentence(field.label)} in {field.unit}"
				class="h-9 min-w-0"
				data-filter-input
				value={values[field.minKey] ?? ''}
				oninput={(e) => onChange({ ...values, [field.minKey]: e.currentTarget.value })}
				onkeydown={onKeydown}
			/>
			<span class="text-muted-foreground" aria-hidden="true">–</span>
			<Input
				inputmode="decimal"
				placeholder="Max"
				aria-label="Maximum {labelInSentence(field.label)} in {field.unit}"
				class="h-9 min-w-0"
				value={values[field.maxKey] ?? ''}
				oninput={(e) => onChange({ ...values, [field.maxKey]: e.currentTarget.value })}
				onkeydown={onKeydown}
			/>
			<span class="text-xs text-muted-foreground">{field.unit}</span>
		</div>
	{:else if field.multiline}
		<textarea
			rows="3"
			placeholder={field.placeholder}
			aria-labelledby="{id}-label"
			aria-invalid={!!error}
			aria-describedby="{id}-note"
			class="flex w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-xs transition-[color,box-shadow] outline-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 dark:bg-input/30 {field.mono
				? 'font-mono text-xs'
				: ''}"
			data-filter-input
			value={values[field.key] ?? ''}
			oninput={(e) => onChange({ ...values, [field.key]: e.currentTarget.value })}
		></textarea>
	{:else}
		<Input
			placeholder={field.placeholder}
			aria-labelledby="{id}-label"
			aria-invalid={!!error}
			aria-describedby="{id}-note"
			class="h-9 {field.mono ? 'font-mono text-xs' : ''}"
			data-filter-input
			value={values[field.key] ?? ''}
			oninput={(e) => onChange({ ...values, [field.key]: e.currentTarget.value })}
			onkeydown={onKeydown}
		/>
	{/if}

	{#if error}
		<p class="text-xs text-destructive" id="{id}-note" role="alert">{error}</p>
	{:else if field.hint}
		<p class="text-xs text-muted-foreground" id="{id}-note">{field.hint}</p>
	{/if}
</div>
