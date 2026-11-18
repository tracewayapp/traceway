<script lang="ts">
	import * as Table from '$lib/components/ui/table';
	import type { BreakdownRow } from '$lib/utils/metric-widgets';
	import { formatMetricLabel } from '$lib/utils/metric-format';

	let {
		rows,
		unit,
		dimension,
		aggregation,
		additive = false
	} = $props<{
		rows: BreakdownRow[];
		unit: string;
		dimension: string;
		aggregation: string;
		/** A counter: the value is the count, so the count column would repeat it. */
		additive?: boolean;
	}>();

	function percent(share: number): string {
		const value = share * 100;
		return `${value >= 10 || value === 0 ? Math.round(value) : value.toFixed(1)}%`;
	}
</script>

{#if rows.length > 0}
	<div class="h-full overflow-auto">
		<Table.Root>
			<Table.Header>
				<Table.Row>
					<Table.Head class="h-8 text-xs">{dimension}</Table.Head>
					<Table.Head class="h-8 text-right text-xs whitespace-nowrap">{aggregation}</Table.Head>
					{#if !additive}
						<Table.Head class="h-8 text-right text-xs">Count</Table.Head>
					{/if}
					<Table.Head class="h-8 text-right text-xs">Share</Table.Head>
				</Table.Row>
			</Table.Header>
			<Table.Body>
				{#each rows as row (row.key)}
					<Table.Row>
						<Table.Cell class="w-full max-w-0 truncate py-1.5 font-mono text-xs" title={row.key}>
							{row.key}
						</Table.Cell>
						<Table.Cell class="py-1.5 text-right text-xs whitespace-nowrap tabular-nums">
							{formatMetricLabel(row.value, unit)}
						</Table.Cell>
						{#if !additive}
							<Table.Cell class="py-1.5 text-right text-xs text-muted-foreground tabular-nums">
								{Math.round(row.count).toLocaleString()}
							</Table.Cell>
						{/if}
						<Table.Cell class="py-1.5">
							<div class="flex items-center justify-end gap-2">
								<div class="h-1.5 w-12 shrink-0 overflow-hidden rounded bg-muted">
									<div
										class="h-full bg-primary"
										style="width: {Math.min(100, row.share * 100)}%"
									></div>
								</div>
								<span class="w-9 text-right text-xs text-muted-foreground tabular-nums"
									>{percent(row.share)}</span
								>
							</div>
						</Table.Cell>
					</Table.Row>
				{/each}
			</Table.Body>
		</Table.Root>
	</div>
{:else}
	<div class="flex h-full items-center justify-center text-sm text-muted-foreground">No data</div>
{/if}
