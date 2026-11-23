import { describe, expect, it } from 'vitest';
import {
	escapeFilterKey,
	filterOperator,
	formatAttributeFilter,
	keyOperatorOptions,
	operatorFlags,
	operatorOptions,
	parseAttributeFilter,
	unescapeFilterKey,
	withOperator,
	type AttributeFilter
} from './attribute-filters';

describe('attribute filter operators', () => {
	it.each(keyOperatorOptions.map((o) => o.value))(
		'round trips %s through its flags',
		(operator) => {
			expect(filterOperator(operatorFlags(operator))).toBe(operator);
		}
	);

	it('maps the flags the backend reads', () => {
		expect(operatorFlags('=')).toEqual({ exclude: false, contains: false, exists: false });
		expect(operatorFlags('!=')).toEqual({ exclude: true, contains: false, exists: false });
		expect(operatorFlags('~=')).toEqual({ exclude: false, contains: true, exists: false });
		expect(operatorFlags('!~=')).toEqual({ exclude: true, contains: true, exists: false });
		expect(operatorFlags('?=')).toEqual({ exclude: false, contains: false, exists: true });
		expect(operatorFlags('!?=')).toEqual({ exclude: true, contains: false, exists: true });
	});

	it('keeps existence checks out of the pages that match values only', () => {
		expect(operatorOptions.map((o) => o.value)).toEqual(['=', '!=', '~=', '!~=']);
		expect(keyOperatorOptions.map((o) => o.value)).toContain('?=');
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
		'note=line 1\nline 2',
		'NodeId?=',
		'NodeId!?=',
		'a!b=1',
		'why?not=1'
	])('round trips %s', (input) => {
		const filter = parseAttributeFilter(input)!;
		expect(filter).not.toBeNull();
		expect(formatAttributeFilter(filter)).toBe(input);
	});

	it('keeps old equality links compatible', () => {
		expect(parseAttributeFilter('userId=u_42')).toEqual({
			key: 'userId',
			value: 'u_42',
			exclude: false,
			contains: false,
			exists: false
		});
	});

	it('drops a value given to an existence check', () => {
		expect(parseAttributeFilter('NodeId?=7')).toMatchObject({
			key: 'NodeId',
			value: '',
			exists: true
		});
	});

	it.each(['', '=missing-key', '  =value', 'no-operator'])('rejects %s', (input) => {
		expect(parseAttributeFilter(input)).toBeNull();
	});
});

describe('attribute filter key escaping', () => {
	const filter = (key: string, operator: '=' | '!=' | '~=' | '?=' = '='): AttributeFilter => ({
		key,
		value: operator === '?=' ? '' : 'v',
		...operatorFlags(operator)
	});

	it.each([
		['a=b', '='],
		['ends!', '='],
		['ends~', '!='],
		['ends?', '~='],
		['really!?', '?='],
		['100%', '='],
		['%3D', '='],
		['=', '=']
	] as const)('keeps the key %s whole with %s', (key, operator) => {
		const f = filter(key, operator);
		expect(parseAttributeFilter(formatAttributeFilter(f))).toEqual(f);
	});

	it('leaves ordinary keys readable', () => {
		expect(escapeFilterKey('http.route')).toBe('http.route');
		expect(escapeFilterKey('a!b~c?d')).toBe('a!b~c?d');
		expect(escapeFilterKey('ends!')).toBe('ends%21');
	});

	it('decodes only the characters it escapes', () => {
		expect(unescapeFilterKey('a%3Db%25%20c')).toBe('a=b%%20c');
	});
});

describe('withOperator', () => {
	const a: AttributeFilter = { key: 'k', value: 'v', ...operatorFlags('=') };
	const b: AttributeFilter = { key: 'k', value: 'v', ...operatorFlags('!=') };

	it('switches one pill in place', () => {
		expect(withOperator([a], 0, '~=')).toEqual([{ ...a, contains: true }]);
	});

	it('returns null when nothing changes', () => {
		expect(withOperator([a], 0, '=')).toBeNull();
	});

	it('drops a pill that would duplicate another', () => {
		expect(withOperator([a, b], 1, '=')).toEqual([a]);
	});
});
