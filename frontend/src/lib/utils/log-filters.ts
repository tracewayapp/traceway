import {
	attributeFiltersEqual,
	formatAttributeFilter,
	parseAttributeFilter,
	type AttributeFilter
} from './attribute-filters';

export type LogAttributeScope = 'resource' | 'scope' | 'log';

export type LogAttributeFilter = AttributeFilter & { scope: LogAttributeScope };

// Parse `resource.service.name=backend-service` or `log.method!=Enter`
// (scope.*/log.* likewise; `~=` / `!~=` for substring match) into a
// structured filter. Returns null if the input doesn't match the shape.
// The value is kept verbatim: a filter made from a tile whose value ends in
// whitespace or a newline must still match it after a reload.
export function parseLogAttributeFilter(input: string): LogAttributeFilter | null {
	// The layer prefix is taken first, so a key that itself starts with an
	// operator character (`log.~=1` is key `~`) is not split at `log.`.
	const m = input.trimStart().match(/^(resource|scope|log)\.(.+)$/s);
	const filter = m && parseAttributeFilter(m[2]);
	if (!filter) return null;
	return { ...filter, scope: m[1] as LogAttributeScope };
}

export function formatLogAttributeFilter(f: LogAttributeFilter): string {
	return `${f.scope}.${formatAttributeFilter(f)}`;
}

// Displayed attribute keys are prefixed `resource.` / `scope.`; log
// attributes are unprefixed.
export function displayKeyToFilter(displayKey: string): { scope: LogAttributeScope; key: string } {
	if (displayKey.startsWith('resource.')) {
		return { scope: 'resource', key: displayKey.slice('resource.'.length) };
	}
	if (displayKey.startsWith('scope.')) {
		return { scope: 'scope', key: displayKey.slice('scope.'.length) };
	}
	return { scope: 'log', key: displayKey };
}

export function sameFilterTarget(
	a: Pick<LogAttributeFilter, 'scope' | 'key' | 'value'>,
	b: Pick<LogAttributeFilter, 'scope' | 'key' | 'value'>
): boolean {
	return a.scope === b.scope && a.key === b.key && a.value === b.value;
}

export function filtersEqual(a: LogAttributeFilter, b: LogAttributeFilter): boolean {
	return a.scope === b.scope && attributeFiltersEqual(a, b);
}
