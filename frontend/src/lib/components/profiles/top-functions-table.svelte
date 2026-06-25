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
	import { formatValue } from '$lib/utils/profile-format';

	interface Props {
		rows: FunctionStat[];
		unit: string;
	}

	let { rows, unit }: Props = $props();

	let sortField = $state<'flat' | 'cum'>('flat');
	let sortDirection = $state<'asc' | 'desc'>('desc');

	function onSort(field: string) {
		const next = field as 'flat' | 'cum';
		if (sortField === next) {
			sortDirection = sortDirection === 'desc' ? 'asc' : 'desc';
		} else {
			sortField = next;
			sortDirection = 'desc';
		}
	}

	const sorted = $derived.by(() => {
		const copy = [...rows];
		copy.sort((a, b) =>
			sortDirection === 'desc' ? b[sortField] - a[sortField] : a[sortField] - b[sortField]
		);
		return copy;
	});

	function pct(value: number): string {
		return `${Number(value.toFixed(1))}%`;
	}
</script>

{#if rows.length}
	<div class="overflow-hidden rounded-lg border" data-testid="top-functions">
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
				{#each sorted as row (row.name)}
					<Table.Row>
						<Table.Cell class="max-w-[28rem] truncate font-mono text-xs" title={row.name}>
							{row.name}
						</Table.Cell>
						<Table.Cell class="text-right tabular-nums">
							{formatValue(unit, row.flat)}
						</Table.Cell>
						<Table.Cell class="text-right tabular-nums text-muted-foreground">
							{pct(row.flatPct)}
						</Table.Cell>
						<Table.Cell class="text-right tabular-nums">
							{formatValue(unit, row.cum)}
						</Table.Cell>
						<Table.Cell>
							<div class="flex items-center justify-end gap-2">
								<div class="h-1.5 w-16 overflow-hidden rounded bg-muted">
									<div class="h-full bg-primary" style="width: {Math.min(100, row.cumPct)}%"></div>
								</div>
								<span class="tabular-nums text-muted-foreground">{pct(row.cumPct)}</span>
							</div>
						</Table.Cell>
					</Table.Row>
				{/each}
			</Table.Body>
		</Table.Root>
	</div>
{/if}
