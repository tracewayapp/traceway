import { describe, expect, it } from 'vitest';
import { formatChartParams, parseChartParams } from './log-attribute-chart';

describe('chart URL params', () => {
	it('defaults to the average and maximum over time', () => {
		const chart = parseChartParams(new URLSearchParams('chart=log.TimerSend'))!;
		expect(chart).toEqual({
			target: { scope: 'log', key: 'TimerSend' },
			view: 'series',
			aggregations: ['avg', 'max']
		});
		expect(formatChartParams(chart)).toEqual({
			chart: 'log.TimerSend',
			chartView: null,
			chartAgg: null
		});
	});

	it('round trips a view and lines in a fixed order', () => {
		const params = new URLSearchParams(
			'chart=resource.host.cpu&chartView=distribution&chartAgg=max,p95,bogus'
		);
		const chart = parseChartParams(params)!;
		expect(chart.view).toBe('distribution');
		expect(chart.aggregations).toEqual(['p95', 'max']);
		expect(formatChartParams(chart)).toEqual({
			chart: 'resource.host.cpu',
			chartView: 'distribution',
			chartAgg: 'p95,max'
		});
	});

	it('ignores a chart without a layer', () => {
		expect(parseChartParams(new URLSearchParams('chart=TimerSend'))).toBeNull();
		expect(formatChartParams(null)).toEqual({ chart: null, chartView: null, chartAgg: null });
	});
});
