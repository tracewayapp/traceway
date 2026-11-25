import {
	attributeFiltersEqual,
	formatAttributeFilter,
	keyOperatorOptions,
	parseAttributeFilter,
	type AttributeFilter
} from './attribute-filters';
import type { AttributeRules } from '$lib/components/traceway/filter-bar/attributes';

export type LogAttributeScope = 'resource' | 'scope' | 'log';

export type LogAttributeFilter = AttributeFilter & { scope: LogAttributeScope };

export type LogAttributeTarget = { scope: LogAttributeScope; key: string };

// Parse `resource.service.name=backend-service` or `log.method!=Enter`
// (scope.*/log.* likewise; `~=` / `!~=` for substring match, `?=` / `!?=`
// for whether the key is there) into a structured filter. Returns null if
// the input doesn't match the shape. The value is kept verbatim: a filter
// made from a tile whose value ends in whitespace or a newline must still
// match it after a reload.
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

export function formatLogAttributeTarget(t: LogAttributeTarget): string {
	return `${t.scope}.${t.key}`;
}

export function parseLogAttributeTarget(input: string): LogAttributeTarget | null {
	const m = input.match(/^(resource|scope|log)\.(.+)$/s);
	return m ? { scope: m[1] as LogAttributeScope, key: m[2] } : null;
}

export type LogAttributeEntries = {
	// Display key -> value, for the attributes grid.
	attributes: Record<string, string>;
	// Display key -> the attribute it shows, for filters and charts.
	targets: Map<string, LogAttributeTarget>;
};

type LogAttributeLayers = {
	resourceAttributes?: Record<string, string> | null;
	scopeAttributes?: Record<string, string> | null;
	logAttributes?: Record<string, string> | null;
};

// Resource and scope attributes are shown prefixed `resource.` / `scope.`,
// log attributes as they are. A log attribute whose own key starts with
// `resource.` or `scope.` is shown as `log.<key>`, so it can neither be
// mistaken for nor collide with an attribute of the other layers.
export function logAttributeEntries(log: LogAttributeLayers): LogAttributeEntries {
	const attributes: Record<string, string> = {};
	const targets = new Map<string, LogAttributeTarget>();
	const push = (
		m: Record<string, string> | null | undefined,
		scope: LogAttributeScope,
		display: (key: string) => string
	) => {
		if (!m) return;
		for (const [key, value] of Object.entries(m)) {
			const shown = display(key);
			attributes[shown] = value;
			targets.set(shown, { scope, key });
		}
	};
	push(log.resourceAttributes, 'resource', (k) => `resource.${k}`);
	push(log.scopeAttributes, 'scope', (k) => `scope.${k}`);
	push(log.logAttributes, 'log', (k) =>
		k.startsWith('resource.') || k.startsWith('scope.') ? `log.${k}` : k
	);
	return { attributes, targets };
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

// The filter bar edits flat keys (`resource.service.name`); the logs page keeps the scope apart.
export function toFlatLogFilter(f: LogAttributeFilter): AttributeFilter {
	return {
		key: formatLogAttributeTarget(f),
		value: f.value,
		exclude: f.exclude,
		contains: f.contains,
		exists: f.exists
	};
}

export function fromFlatLogFilter(f: AttributeFilter): LogAttributeFilter[] {
	const target = parseLogAttributeTarget(f.key);
	return target ? [{ ...f, ...target }] : [];
}

export const LOG_ATTRIBUTE_RULES: AttributeRules = {
	operators: keyOperatorOptions,
	hint: 'Keys start with resource., scope. or log.',
	keyPlaceholder: 'resource.service.name',
	valuePlaceholder: 'backend-service',
	value: 'trim-end',
	key: (key) => {
		const target = parseLogAttributeTarget(key);
		return target && target.key.trim()
			? formatLogAttributeTarget(target)
			: { error: 'Key must start with resource., scope., or log.' };
	}
};
