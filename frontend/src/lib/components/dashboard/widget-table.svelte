<script lang="ts">
	import * as Table from '$lib/components/ui/table';
	import type { MetricTrendPoint } from '$lib/types/dashboard';
	import { formatMetricLabel } from '$lib/utils/metric-format';

	type SeriesItem = {
		key: string;
		data: MetricTrendPoint[];
	};

	let { series = [], unit = '' } = $props<{
		series: SeriesItem[];
		unit?: string;
	}>();

	const stats = $derived(() => {
		const s = series[0];
		if (!s) return [];
		const values = s.data.map((d: MetricTrendPoint) => d.value);
		if (values.length === 0) {
			return [
				{ label: 'Latest', value: 0 },
				{ label: 'Min', value: 0 },
				{ label: 'Max', value: 0 },
				{ label: 'Avg', value: 0 }
			];
		}
		const sum = values.reduce((a: number, b: number) => a + b, 0);
		return [
			{ label: 'Latest', value: values[values.length - 1] },
			{ label: 'Min', value: Math.min(...values) },
			{ label: 'Max', value: Math.max(...values) },
			{ label: 'Avg', value: sum / values.length }
		];
	});

	function fmt(v: number): string {
		if (unit) return formatMetricLabel(v, unit);
		if (Number.isInteger(v)) return v.toString();
		return v.toFixed(2);
	}
</script>

<div class="overflow-auto">
	<Table.Root>
		<Table.Body>
			{#each stats() as stat, __index (__index)}
				<Table.Row>
					<Table.Cell class="font-medium">{stat.label}</Table.Cell>
					<Table.Cell class="text-right">{fmt(stat.value)}</Table.Cell>
				</Table.Row>
			{/each}
		</Table.Body>
	</Table.Root>
</div>
