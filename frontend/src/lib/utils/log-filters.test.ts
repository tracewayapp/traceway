import { describe, expect, it } from 'vitest';
import {
	filtersEqual,
	formatLogAttributeFilter,
	logAttributeEntries,
	parseLogAttributeFilter,
	parseLogAttributeTarget,
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
		'log.NodeId?=',
		'resource.k8s.pod.name!?='
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
			contains: false,
			exists: false
		});
	});

	it('reads old links whose key starts with an operator character', () => {
		expect(parseLogAttributeFilter('log.=a=b')).toMatchObject({ key: '=a', value: 'b' });
		expect(parseLogAttributeFilter('log.~=1')).toMatchObject({ key: '~', value: '1' });
		expect(parseLogAttributeFilter('log.!=1')).toMatchObject({ key: '!', value: '1' });
	});

	it('escapes such keys when it writes them', () => {
		const filter = parseLogAttributeFilter('log.~=1')!;
		expect(formatLogAttributeFilter(filter)).toBe('log.%7E=1');
		expect(parseLogAttributeFilter(formatLogAttributeFilter(filter))).toEqual(filter);
	});

	it.each(['foo=bar', 'log.=value', 'resource.service.name', 'event.name=x'])(
		'rejects %s',
		(input) => {
			expect(parseLogAttributeFilter(input)).toBeNull();
		}
	);
});

describe('logAttributeEntries', () => {
	const entries = logAttributeEntries({
		resourceAttributes: { 'service.name': 'gateway' },
		scopeAttributes: { 'otel.library': 'http' },
		logAttributes: { TimerSend: '12', 'resource.id': 'r-1', 'scope.depth': '3', 'log.level': 'x' }
	});

	it.each([
		['resource.service.name', { scope: 'resource', key: 'service.name' }],
		['scope.otel.library', { scope: 'scope', key: 'otel.library' }],
		['TimerSend', { scope: 'log', key: 'TimerSend' }],
		['log.resource.id', { scope: 'log', key: 'resource.id' }],
		['log.scope.depth', { scope: 'log', key: 'scope.depth' }],
		['log.level', { scope: 'log', key: 'log.level' }]
	])('maps %s', (displayKey, expected) => {
		expect(entries.targets.get(displayKey)).toEqual(expected);
	});

	it('shows a log attribute named like another layer under log.', () => {
		expect(entries.attributes['log.resource.id']).toBe('r-1');
		expect(entries.attributes['resource.id']).toBeUndefined();
	});
});

describe('parseLogAttributeTarget', () => {
	it('reads the layer prefix', () => {
		expect(parseLogAttributeTarget('log.TimerSend')).toEqual({ scope: 'log', key: 'TimerSend' });
		expect(parseLogAttributeTarget('TimerSend')).toBeNull();
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
