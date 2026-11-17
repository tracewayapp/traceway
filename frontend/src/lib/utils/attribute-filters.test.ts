import { describe, expect, it } from 'vitest';
import {
	filterOperator,
	operatorFlags,
	operatorOptions,
	parseAttributeFilter
} from './attribute-filters';

describe('attribute filter operators', () => {
	it.each(operatorOptions.map((o) => o.value))('round trips %s through its flags', (operator) => {
		expect(filterOperator(operatorFlags(operator))).toBe(operator);
	});

	it('maps the flags the backend reads', () => {
		expect(operatorFlags('=')).toEqual({ exclude: false, contains: false });
		expect(operatorFlags('!=')).toEqual({ exclude: true, contains: false });
		expect(operatorFlags('~=')).toEqual({ exclude: false, contains: true });
		expect(operatorFlags('!~=')).toEqual({ exclude: true, contains: true });
	});
});

describe('attribute filter URLs', () => {
	it.each([
		'user.id=u_42',
		'email!=alice@example.com',
		'email~=EXAMPLE',
		'email!~=example',
		'empty=',
		'url=https://example.com/?a=b',
		'note=line 1\nline 2'
	])('round trips %s', (input) => {
		const filter = parseAttributeFilter(input)!;
		expect(filter).not.toBeNull();
		expect(`${filter.key}${filterOperator(filter)}${filter.value}`).toBe(input);
	});
	it('keeps old equality links compatible', () => {
		expect(parseAttributeFilter('userId=u_42')).toEqual({
			key: 'userId',
			value: 'u_42',
			exclude: false,
			contains: false
		});
	});
	it.each(['', '=missing-key', '  =value', 'no-operator'])('rejects %s', (input) => {
		expect(parseAttributeFilter(input)).toBeNull();
	});
});
