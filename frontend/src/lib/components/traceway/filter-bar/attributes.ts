import {
	attributeFiltersEqual,
	operatorFlags,
	operatorOptions,
	type AttributeFilter,
	type FilterOperator,
	type OperatorOption
} from '$lib/utils/attribute-filters';
import type { AttributeConfig } from './fields';

// How a page's attribute filters are checked before they are staged. Each page differs
// only in these rules; the editor, the pills and the drawer are the same everywhere.
export type AttributeRules = {
	operators: readonly OperatorOption[];
	hint?: string;
	keyPlaceholder?: string;
	valuePlaceholder?: string;
	keys?: readonly string[];
	loadValues?: (key: string) => Promise<string[]>;
	// trim: spaces around the value are dropped (span attributes are matched exactly).
	// trim-end: a trailing space typed by mistake is dropped, but a value left as it was
	// (logs: one set from a row tile may really end in whitespace) is kept verbatim.
	// as-typed: the value is stored exactly (session attributes may be empty).
	value?: 'trim' | 'trim-end' | 'as-typed';
	requireValue?: boolean;
	max?: number;
	// One filter per key, for stores that keep a key → value map (profile labels).
	uniqueKeys?: boolean;
	// The key to store, or why the typed key cannot be used.
	key?: (key: string) => string | { error: string };
};

export function parseAttributeInput(
	rules: AttributeRules,
	key: string,
	value: string,
	operator: FilterOperator,
	existing: readonly AttributeFilter[],
	editIndex: number | null = null
): AttributeFilter | string {
	const flags = operatorFlags(operator);
	const typedKey = key.trim();
	if (!typedKey) return 'Enter an attribute key.';
	const checked = rules.key ? rules.key(typedKey) : typedKey;
	if (typeof checked !== 'string') return checked.error;

	const editing = editIndex !== null ? existing[editIndex] : null;
	const mode = rules.value ?? 'trim';
	const kept =
		mode === 'as-typed'
			? value
			: mode === 'trim'
				? value.trim()
				: editing && value === editing.value
					? value
					: value.trimEnd();
	const filter: AttributeFilter = { key: checked, value: flags.exists ? '' : kept, ...flags };

	if (rules.requireValue && !flags.exists && !filter.value) return 'Enter the value to match.';
	const others = existing.filter((_, i) => i !== editIndex);
	if (others.some((other) => attributeFiltersEqual(other, filter))) {
		return 'This filter is already applied.';
	}
	if (rules.uniqueKeys && others.some((other) => other.key === filter.key)) {
		return `${filter.key} already has a filter. Edit that one instead.`;
	}
	if (rules.max !== undefined && editIndex === null && existing.length >= rules.max) {
		return `A search takes at most ${rules.max} attribute filters.`;
	}
	return filter;
}

export function attributeConfig(
	filters: readonly AttributeFilter[],
	rules: AttributeRules
): AttributeConfig {
	return {
		filters,
		operators: rules.operators,
		hint: rules.hint,
		keyPlaceholder: rules.keyPlaceholder,
		valuePlaceholder: rules.valuePlaceholder,
		keys: rules.keys,
		loadValues: rules.loadValues,
		parse: (key, value, operator, existing, editIndex) =>
			parseAttributeInput(rules, key, value, operator, existing, editIndex)
	};
}

// Sessions and endpoint requests: values match as stored (one may really be empty or end in
// a space), and only the value operators apply. noun: what the filters narrow ("sessions").
export function storedValueRules(
	keyPlaceholder: string,
	valuePlaceholder: string,
	noun: string
): AttributeRules {
	return {
		operators: operatorOptions,
		hint: `All filters must match. Contains ignores case; excluded filters also include ${noun} without the attribute.`,
		keyPlaceholder,
		valuePlaceholder,
		value: 'as-typed'
	};
}
