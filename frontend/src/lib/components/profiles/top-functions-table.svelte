<script module lang="ts">
	export interface FunctionStat {
		name: string;
		flat: number;
		cum: number;
		flatPct: number;
		cumPct: number;
	}
</script>

<script lang="ts">
	import * as Table from '$lib/components/ui/table';
	import TracewayTableHeader from '$lib/components/ui/traceway-table-header/traceway-table-header.svelte';
	import { Button } from '$lib/components/ui/button';
	import { formatValue } from '$lib/utils/profile-format';
	import {
		getSortState,
		setSortState,
		handleSortClick,
		type SortDirection
	} from '$lib/utils/sort-storage';

	interface Props {
		rows: FunctionStat[];
		unit: string;
		onSelect?: (name: string) => void;
		baselineRows?: FunctionStat[];
	}

	let { rows, unit, onSelect, baselineRows }: Props = $props();

	const SORT_STORAGE_KEY = 'profile_functions';
	const initialSort = getSortState(SORT_STORAGE_KEY, { field: 'flat', direction: 'desc' });
	let sortField = $state<'flat' | 'cum'>(initialSort.field === 'cum' ? 'cum' : 'flat');
	let sortDirection = $state<SortDirection>(initialSort.direction);
	let expanded = $state(false);

	const COLLAPSED = 15;

	const baseFlat = $derived.by(() => {
		const m = new Map<string, number>();
		for (const r of baselineRows ?? []) m.set(r.name, r.flat);
		return m;
	});

	function onSort(field: string) {
		const newSort = handleSortClick(field, sortField, sortDirection);
		sortField = newSort.field === 'cum' ? 'cum' : 'flat';
		sortDirection = newSort.direction;
		setSortState(SORT_STORAGE_KEY, newSort);
	}

	const sorted = $derived.by(() => {
		const copy = [...rows];
		copy.sort((a, b) =>
			sortDirection === 'desc' ? b[sortField] - a[sortField] : a[sortField] - b[sortField]
		);
		return copy;
	});

	const visible = $derived(expanded ? sorted : sorted.slice(0, COLLAPSED));

	function pct(value: number): string {
		return `${Number(value.toFixed(1))}%`;
	}

	function deltaLabel(row: FunctionStat): { text: string; cls: string } {
		const base = baseFlat.get(row.name) ?? 0;
		const d = row.flat - base;
		if (d === 0) return { text: '—', cls: 'text-muted-foreground' };
		const sign = d > 0 ? '+' : '−';
		const cls =
			d > 0 ? 'text-rose-600 dark:text-rose-400' : 'text-emerald-600 dark:text-emerald-400';
		return { text: `${sign}${formatValue(unit, Math.abs(d))}`, cls };
	}
</script>

{#if rows.length}
	<div class="overflow-hidden rounded-md border" data-testid="top-functions">
		<Table.Root>
			<Table.Header>
				<Table.Row>
					<TracewayTableHeader label="Function" />
					<TracewayTableHeader
						label="Flat"
						tooltip="Time spent inside this function itself (self)"
						align="right"
						sortField="flat"
						currentSortField={sortField}
						{sortDirection}
						{onSort}
					/>
					<TracewayTableHeader label="Flat %" align="right" />
					{#if baselineRows}
						<TracewayTableHeader
							label="Δ Flat"
							tooltip="Change in flat vs baseline"
							align="right"
						/>
					{/if}
					<TracewayTableHeader
						label="Cumulative"
						tooltip="Time spent in this function and everything it calls"
						align="right"
						sortField="cum"
						currentSortField={sortField}
						{sortDirection}
						{onSort}
					/>
					<TracewayTableHeader label="Cum %" align="right" />
				</Table.Row>
			</Table.Header>
			<Table.Body>
				{#each visible as row (row.name)}
					<Table.Row
						class={onSelect ? 'cursor-pointer' : ''}
						onclick={onSelect ? () => onSelect(row.name) : undefined}
					>
						<Table.Cell class="max-w-[28rem] truncate font-mono text-xs" title={row.name}>
							{row.name}
						</Table.Cell>
						<Table.Cell class="text-right tabular-nums">
							{formatValue(unit, row.flat)}
						</Table.Cell>
						<Table.Cell class="text-right text-muted-foreground tabular-nums">
							{pct(row.flatPct)}
						</Table.Cell>
						{#if baselineRows}
							{@const d = deltaLabel(row)}
							<Table.Cell class="text-right tabular-nums {d.cls}">{d.text}</Table.Cell>
						{/if}
						<Table.Cell class="text-right tabular-nums">
							{formatValue(unit, row.cum)}
						</Table.Cell>
						<Table.Cell>
							<div class="flex items-center justify-end gap-2">
								<div class="h-1.5 w-16 shrink-0 overflow-hidden rounded bg-muted">
									<div class="h-full bg-primary" style="width: {Math.min(100, row.cumPct)}%"></div>
								</div>
								<span class="w-12 text-right text-muted-foreground tabular-nums"
									>{pct(row.cumPct)}</span
								>
							</div>
						</Table.Cell>
					</Table.Row>
				{/each}
			</Table.Body>
		</Table.Root>
		{#if sorted.length > COLLAPSED}
			<div class="border-t p-2 text-center">
				<Button variant="ghost" size="sm" onclick={() => (expanded = !expanded)}>
					{expanded ? 'Show less' : `Show all ${sorted.length}`}
				</Button>
			</div>
		{/if}
	</div>
{/if}
