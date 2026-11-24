<script lang="ts">
	import type { Snippet } from 'svelte';
	import { Plus, X } from '@lucide/svelte';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import * as Select from '$lib/components/ui/select';
	import * as Sheet from '$lib/components/ui/sheet';
	import { attributeFiltersEqual, type AttributeFilter } from '$lib/utils/attribute-filters';
	import AttributeChip from './attribute-chip.svelte';
	import AttributeEditor from './attribute-editor.svelte';
	import FieldInput from './field-input.svelte';
	import {
		activeCount,
		anyLabelOf,
		clearedField,
		clearedValues,
		fieldError,
		isFieldSet,
		isSearchField,
		namedDefault,
		optionPill,
		pillOperator,
		pillValue,
		valuesEqual,
		type AttributeConfig,
		type FilterApply,
		type FilterField,
		type FilterOption,
		type FilterValues,
		type SelectField
	} from './fields';

	type Props = {
		// The applied search text. Go (or Enter) runs what is typed.
		search?: string;
		searchPlaceholder?: string;
		// Pages without a search box (a filter-only list) pass false.
		showSearch?: boolean;
		fields?: readonly FilterField[];
		// The applied values of `fields`.
		values?: FilterValues;
		attributes?: AttributeConfig;
		onApply: (next: FilterApply) => void;
		disabled?: boolean;
		// What the filters narrow, for the drawer's subtitle ("spans").
		noun?: string;
		// The time range the page shows, for the drawer's subtitle ("last 1h").
		rangeLabel?: string;
		// Chart, saved filters: page tools at the right end of the pill row.
		actions?: Snippet;
		// The results the filters narrow.
		children?: Snippet;
	};

	let {
		search = '',
		searchPlaceholder = 'Search...',
		showSearch = true,
		fields = [],
		values = {},
		attributes,
		onApply,
		disabled = false,
		noun = 'results',
		rangeLabel,
		actions,
		children
	}: Props = $props();

	const uid = $props.id();
	const searchFields = $derived(fields.filter(isSearchField) as SelectField[]);
	const drawerFields = $derived(fields);
	const pillFields = $derived(fields.filter((f) => !isSearchField(f)));
	const hasFilters = $derived(drawerFields.length > 0 || !!attributes);
	const appliedAttrs = $derived(attributes?.filters ?? []);
	// Quick dropdowns are hidden on phones, so a value set in one shows as a pill there instead.
	const appliedFields = $derived(fields.filter((f) => isFieldSet(f, values)));
	const defaultFields = $derived(
		pillFields.filter((f) => !isFieldSet(f, values) && namedDefault(f))
	);
	const pillCount = $derived(
		appliedFields.filter((f) => !isSearchField(f)).length + appliedAttrs.length
	);

	let draftSearch = $derived(search);
	let draftValues = $derived<FilterValues>({ ...values });
	let draftAttrs = $derived<AttributeFilter[]>([...appliedAttrs]);

	const invalid = $derived(drawerFields.some((f) => fieldError(f, draftValues)));
	const changedFields = $derived(
		new Set(drawerFields.filter((f) => !valuesEqual([f], draftValues, values)).map((f) => f.key))
	);
	const attrChanges = $derived(
		draftAttrs.filter((f) => !appliedAttrs.some((a) => attributeFiltersEqual(a, f))).length +
			appliedAttrs.filter((a) => !draftAttrs.some((f) => attributeFiltersEqual(a, f))).length
	);
	const changeCount = $derived(changedFields.size + attrChanges);
	const draftCount = $derived(activeCount(drawerFields, draftValues, draftAttrs));

	let drawerOpen = $state(false);
	let focusKey = $state<string | null>(null);
	let attrEditor = $state(false);
	let editIndex = $state<number | null>(null);
	let attrSeed = $state<AttributeFilter | null>(null);
	let loadedOptions = $state<Record<string, FilterOption[]>>({});
	let loadingOptions = $state<Record<string, boolean>>({});
	let loadRound = 0;

	const rangeText = $derived(
		!rangeLabel ? '' : rangeLabel.startsWith('the ') ? ` in ${rangeLabel}` : ` in the ${rangeLabel}`
	);

	// Fields whose values come from the data are asked again on every open, so a drawer
	// opened after the time range changed offers what that range holds.
	function loadFieldOptions() {
		const round = ++loadRound;
		for (const field of drawerFields) {
			if (field.kind !== 'select' || !field.loadOptions) continue;
			const key = field.key;
			loadingOptions = { ...loadingOptions, [key]: true };
			field
				.loadOptions()
				.then((options) => {
					if (round === loadRound) loadedOptions = { ...loadedOptions, [key]: options };
				})
				.catch(() => {
					// The field keeps the options it had; the applied value stays pickable.
				})
				.finally(() => {
					if (round === loadRound) loadingOptions = { ...loadingOptions, [key]: false };
				});
		}
	}

	function searchValues(): FilterValues {
		return Object.fromEntries(searchFields.map((f) => [f.key, draftValues[f.key] ?? '']));
	}

	// Go runs the search box with the filters already applied; drawer edits only run from Apply.
	function go() {
		if (disabled) return;
		onApply({
			search: draftSearch.trim(),
			values: { ...values, ...searchValues() },
			attributes: [...appliedAttrs]
		});
	}

	function apply() {
		if (invalid || disabled) return;
		onApply({
			search: draftSearch.trim(),
			values: { ...draftValues },
			attributes: [...draftAttrs]
		});
		drawerOpen = false;
	}

	function resetDrawer() {
		draftValues = { ...values, ...searchValues() };
		draftAttrs = [...appliedAttrs];
		attrEditor = false;
		editIndex = null;
		attrSeed = null;
		focusKey = null;
	}

	function openDrawer(target: { field?: string; attr?: number; seed?: AttributeFilter } = {}) {
		resetDrawer();
		focusKey = target.field ?? null;
		if (target.attr !== undefined) {
			editIndex = target.attr;
			attrEditor = true;
			focusKey = 'attributes';
		} else if (target.seed) {
			attrSeed = target.seed;
			attrEditor = true;
			focusKey = 'attributes';
		}
		drawerOpen = true;
		loadFieldOptions();
	}

	// For a value clicked in the results: the drawer opens with a new attribute filter
	// filled in, so its operator or value can change before Apply.
	export function openAttribute(seed: AttributeFilter) {
		openDrawer({ seed });
	}

	// Opened from a pill: put the cursor in that field instead of on the close button.
	function focusTarget(e: Event) {
		if (!focusKey) return;
		const box = document.getElementById(`${uid}-${focusKey}`);
		const el = box?.querySelector<HTMLElement>('[data-filter-input], input');
		if (box && el) {
			e.preventDefault();
			el.focus({ preventScroll: true });
			box.scrollIntoView({ block: 'nearest' });
		}
	}

	function removeField(field: FilterField) {
		onApply({ search, values: clearedField(field, values), attributes: [...appliedAttrs] });
	}

	function removeAttr(index: number) {
		onApply({
			search,
			values: { ...values },
			attributes: appliedAttrs.filter((_, i) => i !== index)
		});
	}

	function clearFilters() {
		onApply({ search, values: clearedValues(drawerFields, values), attributes: [] });
	}

	function clearDrafts() {
		draftValues = clearedValues(drawerFields, draftValues);
		draftAttrs = [];
		attrEditor = false;
		editIndex = null;
	}

	function submitAttr(filter: AttributeFilter, index: number | null) {
		draftAttrs =
			index === null
				? [...draftAttrs, filter]
				: draftAttrs.map((f, i) => (i === index ? filter : f));
		editIndex = null;
		attrSeed = null;
		attrEditor = false;
	}

	function removeDraftAttr(index: number) {
		draftAttrs = draftAttrs.filter((_, i) => i !== index);
		if (editIndex === index) {
			editIndex = null;
			attrEditor = false;
		}
	}

	function isApplied(f: AttributeFilter) {
		return appliedAttrs.some((a) => attributeFiltersEqual(a, f));
	}

	function selectedOption(field: SelectField) {
		return field.options.find((o) => o.value === draftValues[field.key]);
	}

	function searchFieldLabel(field: SelectField) {
		const v = draftValues[field.key] ?? field.emptyValue ?? '';
		return v === (field.emptyValue ?? '')
			? anyLabelOf(field)
			: (field.options.find((o) => o.value === v)?.label ?? v);
	}
</script>

{#if showSearch}
	<div class="flex flex-wrap items-center gap-2">
		<div class="flex w-full min-w-0 items-center sm:w-auto">
			<Input
				placeholder={searchPlaceholder}
				aria-label={searchPlaceholder}
				class="h-9 min-w-0 flex-1 rounded-r-none border-r-0 shadow-none focus-visible:border-r focus-visible:border-ring focus-visible:ring-0 sm:w-[250px] sm:flex-none lg:w-[320px]"
				value={draftSearch}
				oninput={(e) => (draftSearch = e.currentTarget.value)}
				onkeydown={(e) => {
					if (e.key === 'Enter') go();
				}}
			/>
			{#each searchFields as field (field.key)}
				<Select.Root
					type="single"
					value={draftValues[field.key] ?? field.emptyValue ?? ''}
					onValueChange={(v) => (draftValues = { ...draftValues, [field.key]: v ?? '' })}
				>
					<Select.Trigger
						class="hidden h-9 min-w-[110px] shrink-0 rounded-none border-r-0 shadow-none sm:flex"
						aria-label={field.label}
					>
						<span class="truncate {selectedOption(field)?.class ?? ''}"
							>{searchFieldLabel(field)}</span
						>
					</Select.Trigger>
					<Select.Content>
						<Select.Item value={field.emptyValue ?? ''} label={anyLabelOf(field)}>
							{anyLabelOf(field)}
						</Select.Item>
						{#each field.options.filter((o) => o.value !== (field.emptyValue ?? '')) as option (option.value)}
							<Select.Item value={option.value} label={option.label}>
								<span class={option.class ?? ''}>{option.label}</span>
							</Select.Item>
						{/each}
					</Select.Content>
				</Select.Root>
			{/each}
			<Button
				variant="outline"
				class="h-9 shrink-0 rounded-l-none shadow-none"
				onclick={go}
				{disabled}
			>
				Go
			</Button>
		</div>
		{#if !hasFilters && actions}
			<div class="ml-auto flex items-center gap-2">{@render actions()}</div>
		{/if}
	</div>
{/if}

{#if hasFilters}
	<div class="flex flex-wrap items-center gap-2">
		<button
			type="button"
			class="inline-flex h-6 items-center gap-1 rounded-full border border-dashed border-muted-foreground/60 px-3 text-xs font-medium text-muted-foreground transition-colors hover:border-foreground hover:text-foreground"
			data-filter-add
			onclick={() => openDrawer()}
		>
			<Plus class="h-3 w-3" />
			Add filter
		</button>
		{#each defaultFields as field (field.key)}
			<button
				type="button"
				class="inline-flex h-6 items-center gap-1 rounded-full bg-muted px-2 text-xs hover:text-primary"
				title="Default. Change it in the filters."
				aria-label="Edit filter {field.label} = {namedDefault(field)}"
				data-filter-pill
				onclick={() => openDrawer({ field: field.key })}
			>
				<span class="text-muted-foreground">{field.label}</span>
				<span class="text-muted-foreground">=</span>
				<span>{namedDefault(field)}</span>
			</button>
		{/each}
		{#each appliedFields as field (field.key)}
			{@const op = pillOperator(field, values)}
			{@const own = optionPill(field, values)}
			<span
				class="inline-flex h-6 max-w-full items-center gap-1 rounded-full bg-muted px-2 text-xs {isSearchField(
					field
				)
					? 'sm:hidden'
					: ''}"
			>
				<button
					type="button"
					class="inline-flex h-6 min-w-0 items-center gap-1 hover:text-primary"
					title="Edit filter"
					aria-label={own
						? `Edit filter ${own}`
						: `Edit filter ${field.label} ${op || '='} ${pillValue(field, values)}`}
					data-filter-pill
					onclick={() => openDrawer({ field: field.key })}
				>
					{#if own}
						<span class="truncate">{own}</span>
					{:else}
						<span class="text-muted-foreground">{field.label}</span>
						{#if op}<span class="text-muted-foreground">{op}</span>{/if}
						<span class="truncate {field.mono ? 'font-mono' : ''}">{pillValue(field, values)}</span>
					{/if}
				</button>
				<button
					type="button"
					aria-label="Remove {field.label} filter"
					class="-mr-1.5 inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-full text-muted-foreground hover:text-foreground"
					onclick={() => removeField(field)}
				>
					<X class="h-3 w-3" />
				</button>
			</span>
		{/each}
		{#each appliedAttrs as f, i (i)}
			<AttributeChip
				filter={f}
				onEdit={() => openDrawer({ attr: i })}
				onRemove={() => removeAttr(i)}
			/>
		{/each}
		{#if pillCount > 1}
			<button
				type="button"
				class="h-6 px-1 text-xs text-muted-foreground hover:text-foreground"
				onclick={clearFilters}>Clear all</button
			>
		{/if}
		{#if actions}<div class="ml-auto flex items-center gap-2">{@render actions()}</div>{/if}
	</div>
{/if}

{@render children?.()}

{#if hasFilters}
	<Sheet.Root
		bind:open={drawerOpen}
		onOpenChange={(open) => {
			if (!open) resetDrawer();
		}}
	>
		<Sheet.Content side="right" class="w-full gap-0 sm:max-w-md" onOpenAutoFocus={focusTarget}>
			<Sheet.Header class="border-b">
				<Sheet.Title>Filters</Sheet.Title>
				<Sheet.Description>
					Applied to {noun}{rangeText}
				</Sheet.Description>
			</Sheet.Header>
			<div class="flex-1 space-y-5 overflow-y-auto p-4">
				{#each drawerFields as field (field.key)}
					<FieldInput
						{field}
						id="{uid}-{field.key}"
						highlight={focusKey === field.key}
						changed={changedFields.has(field.key)}
						options={loadedOptions[field.key]}
						loading={loadingOptions[field.key] ?? false}
						values={draftValues}
						onChange={(v) => (draftValues = v)}
						onSubmit={apply}
					/>
				{/each}
				{#if attributes}
					<div
						id="{uid}-attributes"
						class="space-y-2 rounded-md {focusKey === 'attributes'
							? 'ring-2 ring-primary/40 ring-offset-4 ring-offset-background'
							: ''}"
					>
						<span class="text-xs font-medium">Attributes</span>
						<div class="flex flex-wrap items-center gap-2">
							{#each draftAttrs as f, i (i)}
								<AttributeChip
									filter={f}
									pending={!isApplied(f)}
									onEdit={() => {
										editIndex = i;
										attrSeed = null;
										attrEditor = true;
									}}
									onRemove={() => removeDraftAttr(i)}
								/>
							{/each}
							{#if !attrEditor}
								<button
									type="button"
									class="inline-flex h-6 items-center gap-1 rounded-full border border-dashed border-muted-foreground/60 px-3 text-xs font-medium text-muted-foreground transition-colors hover:border-foreground hover:text-foreground"
									onclick={() => {
										editIndex = null;
										attrSeed = null;
										attrEditor = true;
									}}
								>
									<Plus class="h-3 w-3" />
									Add attribute filter
								</button>
							{/if}
						</div>
						{#if attrEditor}
							<div class="rounded-md border bg-muted/30 p-3">
								<p class="mb-2 text-xs text-muted-foreground">
									{editIndex !== null ? 'Edit attribute filter' : 'New attribute filter'}
								</p>
								<AttributeEditor
									config={attributes}
									filters={draftAttrs}
									{editIndex}
									seed={attrSeed}
									onSubmit={submitAttr}
									onCancel={() => {
										attrEditor = false;
										editIndex = null;
										attrSeed = null;
									}}
								/>
							</div>
						{/if}
					</div>
				{/if}
			</div>
			<Sheet.Footer class="flex-row items-center justify-between gap-2 border-t">
				{#if draftCount > 0}
					<Button variant="ghost" class="h-9" onclick={clearDrafts}>Clear all</Button>
				{:else}
					<span></span>
				{/if}
				<Button class="h-9" onclick={apply} {disabled}>
					{changeCount > 0 ? `Apply ${changeCount} change${changeCount === 1 ? '' : 's'}` : 'Apply'}
				</Button>
			</Sheet.Footer>
		</Sheet.Content>
	</Sheet.Root>
{/if}
