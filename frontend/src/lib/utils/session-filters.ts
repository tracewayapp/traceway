export type AttributeFilter = {
	key: string;
	value: string;
	exclude: boolean;
	contains: boolean;
};

export type FilterOperator = '=' | '!=' | '~=' | '!~=';

export function filterOperator(filter: AttributeFilter): FilterOperator {
	if (filter.contains) return filter.exclude ? '!~=' : '~=';
	return filter.exclude ? '!=' : '=';
}

export function parseAttributeFilter(input: string): AttributeFilter | null {
	const match = input.match(/^(.+?)(!~=|~=|!=|=)(.*)$/s);
	if (!match || !match[1].trim()) return null;
	return {
		key: match[1].trim(),
		value: match[3],
		exclude: match[2].startsWith('!'),
		contains: match[2].includes('~')
	};
}
