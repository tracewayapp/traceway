export type FilterOperator = '=' | '!=' | '~=' | '!~=' | '?=' | '!?=';

export type OperatorFlags = { exclude: boolean; contains: boolean; exists: boolean };

export type OperatorOption = {
	value: FilterOperator;
	symbol: string;
	label: string;
	// What a pill shows in place of the operator.
	pill: string;
};

const equalityOperators: readonly OperatorOption[] = [
	{ value: '=', symbol: '=', label: 'Equals', pill: '=' },
	{ value: '!=', symbol: '≠', label: 'Not equals', pill: '!=' },
	{ value: '~=', symbol: '~', label: 'Contains', pill: '~=' },
	{ value: '!~=', symbol: '!~', label: 'Not contains', pill: '!~=' }
];

const existenceOperators: readonly OperatorOption[] = [
	{ value: '?=', symbol: '∃', label: 'Key exists', pill: 'exists' },
	{ value: '!?=', symbol: '∄', label: 'Key does not exist', pill: 'not exists' }
];

// Sessions and endpoint traces match on values only.
export const operatorOptions: readonly OperatorOption[] = equalityOperators;

// Logs can also be filtered on whether a key is there at all.
export const logOperatorOptions: readonly OperatorOption[] = [
	...equalityOperators,
	...existenceOperators
];

export function operatorOption(operator: FilterOperator): OperatorOption {
	return logOperatorOptions.find((o) => o.value === operator) ?? logOperatorOptions[0];
}

export function isExistenceOperator(operator: FilterOperator): boolean {
	return operator === '?=' || operator === '!?=';
}

export function filterOperator(flags: OperatorFlags): FilterOperator {
	if (flags.exists) return flags.exclude ? '!?=' : '?=';
	if (flags.contains) return flags.exclude ? '!~=' : '~=';
	return flags.exclude ? '!=' : '=';
}

export function operatorFlags(operator: FilterOperator): OperatorFlags {
	return {
		exclude: operator.startsWith('!'),
		contains: operator.includes('~'),
		exists: isExistenceOperator(operator)
	};
}

export type AttributeFilter = OperatorFlags & { key: string; value: string };

const attributeFilterPattern = /^(.+?)(!~=|!\?=|~=|\?=|!=|=)(.*)$/s;

const keyEscapes: Record<string, string> = {
	'%': '%25',
	'=': '%3D',
	'!': '%21',
	'~': '%7E',
	'?': '%3F'
};

// The key of `key<op>value` ends at the first operator, so a key holding `=`,
// or ending in `!`, `~` or `?`, would be split in the wrong place. Those
// characters are percent-escaped in the key (and `%` itself, so decoding is
// unambiguous); every other key reads as it is, old links included.
export function escapeFilterKey(key: string): string {
	const escaped = key.replace(/[%=]/g, (c) => keyEscapes[c]);
	const tail = escaped.match(/[!~?]+$/);
	if (!tail) return escaped;
	return escaped.slice(0, tail.index) + tail[0].replace(/[!~?]/g, (c) => keyEscapes[c]);
}

export function unescapeFilterKey(key: string): string {
	return key.replace(/%(25|3D|21|7E|3F)/gi, (_, hex: string) =>
		String.fromCharCode(parseInt(hex, 16))
	);
}

// `key<op>value` as it appears in `attr=` URL params. The key ends at the
// first operator, so values may contain `=` and newlines. An existence
// filter (`key?=`, `key!?=`) has no value.
export function parseAttributeFilter(input: string): AttributeFilter | null {
	const match = input.match(attributeFilterPattern);
	if (!match || !match[1].trim()) return null;
	const flags = operatorFlags(match[2] as FilterOperator);
	return {
		key: unescapeFilterKey(match[1].trim()),
		value: flags.exists ? '' : match[3],
		...flags
	};
}

export function formatAttributeFilter(f: AttributeFilter): string {
	return `${escapeFilterKey(f.key)}${filterOperator(f)}${f.exists ? '' : f.value}`;
}

export function attributeFiltersEqual(a: AttributeFilter, b: AttributeFilter): boolean {
	return (
		a.key === b.key &&
		filterOperator(a) === filterOperator(b) &&
		(a.exists ? true : a.value === b.value)
	);
}

// The filter list after one pill switched operator in place, or null when it
// already had that operator. A switch that makes it equal to another pill
// drops it instead of keeping two copies.
export function withOperator<T extends AttributeFilter>(
	filters: readonly T[],
	index: number,
	operator: FilterOperator
): T[] | null {
	const current = filters[index];
	if (!current || filterOperator(current) === operator) return null;
	const flags = operatorFlags(operator);
	const changed = { ...current, ...flags, value: flags.exists ? '' : current.value };
	const others = filters.filter((_, i) => i !== index);
	return others.some((f) => attributeFiltersEqual(f, changed))
		? others
		: filters.map((f, i) => (i === index ? changed : f));
}
