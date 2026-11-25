// What the charts of the logs and spans pages share: which lines a chart over
// time can draw, how a chart's view and lines live in the URL, and how numbers
// and durations read on an axis.

// The backend charts JSON numbers only (shared.ChartNumberPattern), so the
// "Chart values" action is offered for exactly those values.
const numberPattern = /^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][-+]?[0-9]+)?$/;

// A value of the right shape can still be too large for a float ("4e98213"),
// which the backend leaves out as well.
export function isNumericAttributeValue(value: string): boolean {
	return numberPattern.test(value) && Number.isFinite(Number(value));
}

export type ChartView = 'series' | 'distribution';

export type ChartAggregation = 'avg' | 'min' | 'max' | 'sum' | 'count' | 'p50' | 'p95' | 'p99';

export const chartAggregations: readonly {
	value: ChartAggregation;
	label: string;
	color: string;
}[] = [
	{ value: 'avg', label: 'Avg', color: 'var(--chart-1)' },
	{ value: 'p50', label: 'P50', color: 'var(--chart-2)' },
	{ value: 'p95', label: 'P95', color: 'var(--chart-3)' },
	{ value: 'p99', label: 'P99', color: 'var(--chart-4)' },
	{ value: 'min', label: 'Min', color: 'var(--chart-8)' },
	{ value: 'max', label: 'Max', color: 'var(--chart-5)' },
	{ value: 'sum', label: 'Sum', color: 'var(--chart-6)' },
	{ value: 'count', label: 'Count', color: 'var(--chart-7)' }
];

export type ChartLines = { view: ChartView; aggregations: ChartAggregation[] };

// `chartView=distribution`, `chartAgg=avg,p95`; defaults are left out of the URL.
export function parseChartLines(
	params: URLSearchParams,
	defaults: readonly ChartAggregation[]
): ChartLines {
	const view: ChartView = params.get('chartView') === 'distribution' ? 'distribution' : 'series';
	const known = new Set(chartAggregations.map((a) => a.value));
	const picked = (params.get('chartAgg') ?? '')
		.split(',')
		.filter((a): a is ChartAggregation => known.has(a as ChartAggregation));
	return { view, aggregations: picked.length > 0 ? orderAggregations(picked) : [...defaults] };
}

export function formatChartLines(
	lines: ChartLines,
	defaults: readonly ChartAggregation[]
): { chartView: string | null; chartAgg: string | null } {
	const isDefault =
		lines.aggregations.length === defaults.length &&
		defaults.every((a) => lines.aggregations.includes(a));
	return {
		chartView: lines.view === 'distribution' ? 'distribution' : null,
		chartAgg: isDefault ? null : orderAggregations(lines.aggregations).join(',')
	};
}

export function orderAggregations(list: readonly ChartAggregation[]): ChartAggregation[] {
	const set = new Set(list);
	return chartAggregations.map((a) => a.value).filter((a) => set.has(a));
}

export function toggleAggregation(
	list: readonly ChartAggregation[],
	aggregation: ChartAggregation
): ChartAggregation[] {
	if (list.includes(aggregation)) {
		// A chart always draws at least one line.
		return list.length === 1 ? [...list] : list.filter((a) => a !== aggregation);
	}
	return orderAggregations([...list, aggregation]);
}

// Compact numbers for axis ticks, tooltips and the stats row: 12.3k, 4.56M,
// 0.0123, -2.5.
export function formatChartNumber(value: number): string {
	if (!Number.isFinite(value)) return '—';
	const abs = Math.abs(value);
	if (abs >= 1e9) return trimZeros((value / 1e9).toFixed(2)) + 'B';
	if (abs >= 1e6) return trimZeros((value / 1e6).toFixed(2)) + 'M';
	if (abs >= 1e4) return trimZeros((value / 1e3).toFixed(1)) + 'k';
	if (abs >= 100 || Number.isInteger(value)) return String(Math.round(value * 10) / 10);
	if (abs >= 1) return trimZeros(value.toFixed(2));
	if (abs === 0) return '0';
	return trimZeros(value.toPrecision(3));
}

// Durations given in milliseconds, in the unit that reads best: 850µs, 2.4ms,
// 45ms, 1.25s, 2.5m.
export function formatChartMs(ms: number): string {
	if (!Number.isFinite(ms)) return '—';
	const abs = Math.abs(ms);
	if (abs === 0) return '0';
	if (abs < 1) return formatChartNumber(Number((ms * 1000).toPrecision(3))) + 'µs';
	if (abs < 10) return trimZeros(ms.toFixed(2)) + 'ms';
	if (abs < 100) return trimZeros(ms.toFixed(1)) + 'ms';
	if (abs < 1000) return Math.round(ms) + 'ms';
	if (abs < 60_000) return trimZeros((ms / 1000).toFixed(2)) + 's';
	return trimZeros((ms / 60_000).toFixed(1)) + 'm';
}

function trimZeros(s: string): string {
	return s.includes('.') ? s.replace(/\.?0+$/, '') : s;
}
