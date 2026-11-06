import { describe, expect, it } from 'vitest';
import {
	displayKeyToFilter,
	filtersEqual,
	formatLogAttributeFilter,
	parseLogAttributeFilter,
	sameFilterTarget
} from './log-filters';

describe('log attribute filter URLs', () => {
	it.each([
		'resource.service.name=backend',
		'log.method!=Enter',
		'scope.otel.library~=http',
		'log.http.route!~=/health',
		'log.empty=',
		'log.url=https://example.com/?a=b',
		'log.exception.stacktrace=line 1\nline 2\n',
		'log.message!=ends with a space ',
		'log.~=1',
		'log.!=1'
	])('round trips %s', (input) => {
		const filter = parseLogAttributeFilter(input)!;
		expect(filter).not.toBeNull();
		expect(formatLogAttributeFilter(filter)).toBe(input);
	});

	it('splits on the first operator, keeping dotted keys and = in values', () => {
		expect(parseLogAttributeFilter('log.a.b=c=d')).toEqual({
			scope: 'log',
			key: 'a.b',
			value: 'c=d',
			exclude: false,
			contains: false
		});
	});

	it('reads a key that starts with an operator character as part of the key', () => {
		expect(parseLogAttributeFilter('log.=a=b')).toEqual({
			scope: 'log',
			key: '=a',
			value: 'b',
			exclude: false,
			contains: false
		});
	});

	it.each(['foo=bar', 'log.=value', 'resource.service.name', 'event.name=x'])(
		'rejects %s',
		(input) => {
			expect(parseLogAttributeFilter(input)).toBeNull();
		}
	);
});

describe('displayKeyToFilter', () => {
	it.each([
		['resource.service.name', { scope: 'resource', key: 'service.name' }],
		['scope.otel.library', { scope: 'scope', key: 'otel.library' }],
		['TimerSend', { scope: 'log', key: 'TimerSend' }],
		['http.route', { scope: 'log', key: 'http.route' }]
	])('maps %s', (displayKey, expected) => {
		expect(displayKeyToFilter(displayKey)).toEqual(expected);
	});
});

describe('filter comparison', () => {
	const equals = parseLogAttributeFilter('log.TimerSend=7')!;
	const notEquals = parseLogAttributeFilter('log.TimerSend!=7')!;

	it('treats the operator as part of equality but not of the target', () => {
		expect(sameFilterTarget(equals, notEquals)).toBe(true);
		expect(filtersEqual(equals, notEquals)).toBe(false);
		expect(filtersEqual(equals, { ...equals })).toBe(true);
	});

	it('distinguishes scopes', () => {
		const resource = parseLogAttributeFilter('resource.TimerSend=7')!;
		expect(sameFilterTarget(equals, resource)).toBe(false);
	});
});
