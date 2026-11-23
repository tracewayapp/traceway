import {
	formatLogAttributeTarget,
	parseLogAttributeTarget,
	type LogAttributeTarget
} from './log-filters';

// The backend charts JSON numbers only (shared.LogAttributeNumberPattern), so
// the "Chart values" action is offered for exactly those values.
const numberPattern = /^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][-+]?[0-9]+)?$/;

export function isNumericAttributeValue(value: string): boolean {
	return numberPattern.test(value);
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

export const defaultChartAggregations: readonly ChartAggregation[] = ['avg', 'max'];

export type LogAttributeChart = {
	target: LogAttributeTarget;
	view: ChartView;
	aggregations: ChartAggregation[];
};

// `chart=log.TimerSend`, `chartView=distribution`, `chartAgg=avg,p95`.
export function parseChartParams(params: URLSearchParams): LogAttributeChart | null {
	const target = parseLogAttributeTarget(params.get('chart') ?? '');
	if (!target) return null;
	const view: ChartView = params.get('chartView') === 'distribution' ? 'distribution' : 'series';
	const known = new Set(chartAggregations.map((a) => a.value));
	const picked = (params.get('chartAgg') ?? '')
		.split(',')
		.filter((a): a is ChartAggregation => known.has(a as ChartAggregation));
	const aggregations =
		picked.length > 0 ? orderAggregations(picked) : [...defaultChartAggregations];
	return { target, view, aggregations };
}

export function formatChartParams(chart: LogAttributeChart | null): {
	chart: string | null;
	chartView: string | null;
	chartAgg: string | null;
} {
	if (!chart) return { chart: null, chartView: null, chartAgg: null };
	const isDefault =
		chart.aggregations.length === defaultChartAggregations.length &&
		defaultChartAggregations.every((a) => chart.aggregations.includes(a));
	return {
		chart: formatLogAttributeTarget(chart.target),
		chartView: chart.view === 'distribution' ? 'distribution' : null,
		chartAgg: isDefault ? null : orderAggregations(chart.aggregations).join(',')
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

function trimZeros(s: string): string {
	return s.includes('.') ? s.replace(/\.?0+$/, '') : s;
}
