import { describe, expect, it } from 'vitest';
import {
	formatChartNumber,
	formatChartParams,
	isNumericAttributeValue,
	parseChartParams,
	toggleAggregation
} from './log-attribute-chart';

describe('isNumericAttributeValue', () => {
	it.each(['0', '12', '-3', '12.5', '1e3', '-2.5E-4'])('charts %s', (v) => {
		expect(isNumericAttributeValue(v)).toBe(true);
	});
	// The backend's grammar: JSON numbers only.
	it.each(['', 'abc', '007', ' 12', '12 ', '1.', '.5', '+1', 'NaN', 'Infinity', '0x1A'])(
		'skips %j',
		(v) => {
			expect(isNumericAttributeValue(v)).toBe(false);
		}
	);
});

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

describe('toggleAggregation', () => {
	it('adds in display order and never removes the last line', () => {
		expect(toggleAggregation(['max'], 'avg')).toEqual(['avg', 'max']);
		expect(toggleAggregation(['avg', 'max'], 'avg')).toEqual(['max']);
		expect(toggleAggregation(['max'], 'max')).toEqual(['max']);
	});
});

describe('formatChartNumber', () => {
	it.each([
		[0, '0'],
		[12, '12'],
		[-2.5, '-2.5'],
		[12.345, '12.35'],
		[1234, '1234'],
		[12345, '12.3k'],
		[4560000, '4.56M'],
		[0.01234, '0.0123'],
		[Number.NaN, '—']
	])('formats %d as %s', (value, expected) => {
		expect(formatChartNumber(value)).toBe(expected);
	});
});
