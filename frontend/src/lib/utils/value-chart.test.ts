import { describe, expect, it } from 'vitest';
import {
	formatChartLines,
	formatChartMs,
	formatChartNumber,
	isNumericAttributeValue,
	parseChartLines,
	toggleAggregation
} from './value-chart';

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

describe('chart lines in the URL', () => {
	it('leaves the defaults out', () => {
		const lines = parseChartLines(new URLSearchParams(''), ['p50', 'p95']);
		expect(lines).toEqual({ view: 'series', aggregations: ['p50', 'p95'] });
		expect(formatChartLines(lines, ['p50', 'p95'])).toEqual({ chartView: null, chartAgg: null });
	});

	it('keeps a chosen view and lines in display order', () => {
		const lines = parseChartLines(
			new URLSearchParams('chartView=distribution&chartAgg=count,p99,nope'),
			['avg']
		);
		expect(lines).toEqual({ view: 'distribution', aggregations: ['p99', 'count'] });
		expect(formatChartLines(lines, ['avg'])).toEqual({
			chartView: 'distribution',
			chartAgg: 'p99,count'
		});
	});
});

describe('formatChartMs', () => {
	it.each([
		[0, '0'],
		[0.85, '850µs'],
		[0.0123, '12.3µs'],
		[2.4, '2.4ms'],
		[6.789, '6.79ms'],
		[32.58, '32.6ms'],
		[45, '45ms'],
		[123.45, '123ms'],
		[1250, '1.25s'],
		[150000, '2.5m'],
		[Number.NaN, '—']
	])('formats %d ms as %s', (value, expected) => {
		expect(formatChartMs(value)).toBe(expected);
	});
});
