import {
	formatLogAttributeTarget,
	parseLogAttributeTarget,
	type LogAttributeTarget
} from './log-filters';
import {
	formatChartLines,
	parseChartLines,
	type ChartAggregation,
	type ChartLines
} from './value-chart';

export const defaultChartAggregations: readonly ChartAggregation[] = ['avg', 'max'];

export type LogAttributeChart = ChartLines & { target: LogAttributeTarget };

// `chart=log.TimerSend`, `chartView=distribution`, `chartAgg=avg,p95`.
export function parseChartParams(params: URLSearchParams): LogAttributeChart | null {
	const target = parseLogAttributeTarget(params.get('chart') ?? '');
	if (!target) return null;
	return { target, ...parseChartLines(params, defaultChartAggregations) };
}

export function formatChartParams(chart: LogAttributeChart | null): {
	chart: string | null;
	chartView: string | null;
	chartAgg: string | null;
} {
	if (!chart) return { chart: null, chartView: null, chartAgg: null };
	return {
		chart: formatLogAttributeTarget(chart.target),
		...formatChartLines(chart, defaultChartAggregations)
	};
}
