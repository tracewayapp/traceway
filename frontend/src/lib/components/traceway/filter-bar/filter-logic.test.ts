import { describe, expect, it } from 'vitest';
import { keyOperatorOptions, operatorFlags } from '$lib/utils/attribute-filters';
import { parseAttributeInput, storedValueRules, type AttributeRules } from './attributes';
import {
	activeCount,
	anyLabelOf,
	clearedValues,
	isFieldSet,
	namedDefault,
	pillOperator,
	pillValue,
	valuesEqual,
	type FilterField
} from './fields';
import { describeTimeRange, issueTypeField, rootField, traceIdError } from './presets';

const duration: FilterField = {
	kind: 'range',
	key: 'duration',
	label: 'Duration',
	minKey: 'minMs',
	maxKey: 'maxMs',
	unit: 'ms'
};
const kind: FilterField = {
	kind: 'select',
	key: 'kind',
	label: 'Kind',
	options: [{ value: '2', label: 'Server' }]
};

describe('fields', () => {
	it('treats the empty value, blank text and blank ranges as unset', () => {
		expect(isFieldSet(rootField('runs'), { rootFilter: 'all' })).toBe(false);
		expect(isFieldSet(rootField('runs'), { rootFilter: 'root' })).toBe(true);
		expect(isFieldSet(duration, { minMs: ' ', maxMs: '' })).toBe(false);
		expect(isFieldSet(duration, { minMs: '', maxMs: '500' })).toBe(true);
	});

	it('writes a range pill as a bound or a span', () => {
		expect([
			pillOperator(duration, { minMs: '150' }),
			pillValue(duration, { minMs: '150' })
		]).toEqual(['≥', '150 ms']);
		expect(pillOperator(duration, { maxMs: '9' })).toBe('≤');
		expect([
			pillOperator(duration, { minMs: '1', maxMs: '9' }),
			pillValue(duration, { minMs: '1', maxMs: '9' })
		]).toEqual(['', '1–9 ms']);
		expect([pillOperator(kind, { kind: '2' }), pillValue(kind, { kind: '2' })]).toEqual([
			'=',
			'Server'
		]);
	});

	it('clears every field to its own empty value and compares trimmed values', () => {
		const fields = [rootField('runs'), duration, kind];
		const cleared = clearedValues(fields, {
			rootFilter: 'root',
			minMs: '5',
			kind: '2',
			other: 'x'
		});
		expect(cleared).toEqual({ rootFilter: 'all', minMs: '', maxMs: '', kind: '', other: 'x' });
		expect(valuesEqual(fields, { minMs: ' 5' }, { minMs: '5' })).toBe(true);
		expect(activeCount(fields, { rootFilter: 'root', maxMs: '3' }, [])).toBe(2);
	});

	it('reads a label inside a sentence, keeping acronyms', () => {
		expect(anyLabelOf(kind)).toBe('Any kind');
		expect(anyLabelOf({ ...kind, label: 'OS' })).toBe('Any OS');
		expect(anyLabelOf({ ...kind, label: 'SLI kind' })).toBe('Any SLI kind');
		expect(anyLabelOf(rootField('runs'))).toBe('Root and non-root');
	});

	it('names a default only when it narrows the list', () => {
		expect(namedDefault(issueTypeField())).toBeNull();
		expect(
			namedDefault({
				...kind,
				emptyValue: 'active',
				anyLabel: 'Active',
				narrowsByDefault: true
			})
		).toBe('Active');
	});
});

describe('parseAttributeInput', () => {
	const strict: AttributeRules = {
		operators: keyOperatorOptions,
		value: 'trim',
		requireValue: true,
		max: 2,
		key: (k) => (k.includes('"') ? { error: 'No quotes.' } : k)
	};
	const eq = (key: string, value: string) => ({ key, value, ...operatorFlags('=') });

	it('trims, checks the key and requires a value for value operators', () => {
		expect(parseAttributeInput(strict, ' a ', ' 1 ', '=', [])).toEqual(eq('a', '1'));
		expect(parseAttributeInput(strict, ' ', '1', '=', [])).toBe('Enter an attribute key.');
		expect(parseAttributeInput(strict, 'a"', '1', '=', [])).toBe('No quotes.');
		expect(parseAttributeInput(strict, 'a', ' ', '=', [])).toMatch(/value/);
		expect(parseAttributeInput(strict, 'a', 'ignored', '?=', [])).toMatchObject({
			key: 'a',
			value: '',
			exists: true
		});
	});

	it('refuses copies and too many filters, but lets an edit keep its place', () => {
		const list = [eq('a', '1'), eq('b', '2')];
		expect(parseAttributeInput(strict, 'a', '1', '=', list)).toMatch(/already/);
		expect(parseAttributeInput(strict, 'c', '3', '=', list)).toMatch(/at most 2/);
		expect(parseAttributeInput(strict, 'a', '9', '=', list, 0)).toEqual(eq('a', '9'));
		expect(parseAttributeInput(strict, 'b', '2', '=', list, 0)).toMatch(/already/);
	});

	it('keeps stored values as typed, and an unchanged trailing space under trim-end', () => {
		expect(parseAttributeInput(storedValueRules('k', 'v', 'rows'), 'k', ' x ', '=', [])).toEqual(
			eq('k', ' x ')
		);
		const trimEnd: AttributeRules = { operators: keyOperatorOptions, value: 'trim-end' };
		expect(parseAttributeInput(trimEnd, 'k', 'x ', '=', [])).toEqual(eq('k', 'x'));
		expect(parseAttributeInput(trimEnd, 'k', 'x ', '=', [eq('k', 'x ')], 0)).toEqual(eq('k', 'x '));
	});

	it('allows one filter per key when the store is a map', () => {
		const unique: AttributeRules = { operators: keyOperatorOptions, uniqueKeys: true };
		expect(parseAttributeInput(unique, 'k', '2', '=', [eq('k', '1')])).toMatch(/already has/);
		expect(parseAttributeInput(unique, 'k', '2', '=', [eq('k', '1')], 0)).toEqual(eq('k', '2'));
	});
});

describe('presets', () => {
	it('accepts what the backend normalizes as a trace ID', () => {
		expect(traceIdError('4BF92F3577B34DA6A3CE929D0E0E4736')).toBeNull();
		expect(traceIdError('4bf92f35-77b3-4da6-a3ce-929d0e0e4736')).toBeNull();
		expect(traceIdError('123')).toMatch(/32 hexadecimal/);
		expect(traceIdError('0'.repeat(32))).toMatch(/32 hexadecimal/);
	});

	it('describes preset and custom ranges', () => {
		expect(describeTimeRange('24h')).toBe('last 24 hours');
		expect(describeTimeRange('1M')).toBe('last 1 month');
		expect(describeTimeRange(null)).toBe('the custom range');
	});
});
