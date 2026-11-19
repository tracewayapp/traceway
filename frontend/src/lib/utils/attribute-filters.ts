export type FilterOperator = '=' | '!=' | '~=' | '!~=';

export type OperatorFlags = { exclude: boolean; contains: boolean };

export type OperatorOption = { value: FilterOperator; symbol: string; label: string };

export const operatorOptions: readonly OperatorOption[] = [
	{ value: '=', symbol: '=', label: 'Equals' },
	{ value: '!=', symbol: '≠', label: 'Not equals' },
	{ value: '~=', symbol: '~', label: 'Contains' },
	{ value: '!~=', symbol: '!~', label: 'Not contains' }
];

export function filterOperator(flags: OperatorFlags): FilterOperator {
	if (flags.contains) return flags.exclude ? '!~=' : '~=';
	return flags.exclude ? '!=' : '=';
}

export function operatorFlags(operator: FilterOperator): OperatorFlags {
	return { exclude: operator.startsWith('!'), contains: operator.includes('~') };
}

export type AttributeFilter = OperatorFlags & { key: string; value: string };

const attributeFilterPattern = /^(.+?)(!~=|~=|!=|=)(.*)$/s;

// `key<op>value` as it appears in `attr=` URL params. The key ends at the
// first operator, so values may contain `=` and newlines.
export function parseAttributeFilter(input: string): AttributeFilter | null {
	const match = input.match(attributeFilterPattern);
	if (!match || !match[1].trim()) return null;
	return {
		key: match[1].trim(),
		value: match[3],
		...operatorFlags(match[2] as FilterOperator)
	};
}

export function formatAttributeFilter(f: AttributeFilter): string {
	return `${f.key}${filterOperator(f)}${f.value}`;
}

export function attributeFiltersEqual(a: AttributeFilter, b: AttributeFilter): boolean {
	return (
		a.key === b.key && a.value === b.value && a.exclude === b.exclude && a.contains === b.contains
	);
}
