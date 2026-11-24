import {
	attributeFiltersEqual,
	operatorOption,
	filterOperator,
	type AttributeFilter,
	type FilterOperator,
	type OperatorOption
} from '$lib/utils/attribute-filters';

export type FilterOption = {
	value: string;
	label: string;
	count?: number;
	// The whole pill when this option is applied, for options whose label would repeat the
	// field's name ("Root only" rather than "Root = Root").
	pill?: string;
	// Classes for the option's label wherever it is shown (HTTP method colours).
	class?: string;
};

type FieldBase = {
	key: string;
	label: string;
	// One line under the field in the drawer.
	hint?: string;
	mono?: boolean;
};

export type SelectField = FieldBase & {
	kind: 'select';
	options: readonly FilterOption[];
	// The value that means "no filter". Pages that keep `all` in the URL pass it here.
	emptyValue?: string;
	// What the field reads as while unset, such as "All services".
	anyLabel?: string;
	// A quick dropdown in the search bar, between the box and Go, for the filters people
	// change most (Logs level, Issues type, Endpoints method). It is also a field in the
	// drawer, but never a pill: the dropdown already shows its value.
	inSearch?: boolean;
	// The unset value still narrows the list (On-call shows active pages), so a pill names it.
	narrowsByDefault?: boolean;
	// Options that depend on the data (dashboard releases, instances seen in the range) are
	// fetched each time the drawer opens; `options` is what shows until they arrive.
	loadOptions?: () => Promise<FilterOption[]>;
	// A long list gets a search box. Lists over SEARCHABLE_AFTER options get one anyway.
	searchable?: boolean;
};

export const SEARCHABLE_AFTER = 10;

export type RangeField = FieldBase & {
	kind: 'range';
	minKey: string;
	maxKey: string;
	unit: string;
};

export type TextField = FieldBase & {
	kind: 'text';
	placeholder?: string;
	// An error message for a value the backend would refuse, shown once, under the field.
	validate?: (value: string) => string | null;
};

export type FilterField = SelectField | RangeField | TextField;

export type FilterValues = Record<string, string>;

export type AttributeConfig = {
	filters: readonly AttributeFilter[];
	operators: readonly OperatorOption[];
	keyPlaceholder?: string;
	valuePlaceholder?: string;
	// Suggestions for the key box, and for the value box once a key is typed.
	keys?: readonly string[];
	loadValues?: (key: string) => Promise<string[]>;
	// Returns the filter to keep, or the error to show in the editor.
	parse: (
		key: string,
		value: string,
		operator: FilterOperator,
		existing: AttributeFilter[],
		editIndex: number | null
	) => AttributeFilter | string;
};

export type FilterApply = {
	search: string;
	values: FilterValues;
	attributes: AttributeFilter[];
};

const empty = (field: SelectField) => field.emptyValue ?? '';

export function anyLabelOf(field: FilterField): string {
	return (field.kind === 'select' && field.anyLabel) || `Any ${field.label.toLowerCase()}`;
}

// The applied option's own pill text, when it has one.
export function optionPill(field: FilterField, values: FilterValues): string | null {
	if (field.kind !== 'select') return null;
	return field.options.find((o) => o.value === values[field.key])?.pill ?? null;
}

export function isFieldSet(field: FilterField, values: FilterValues): boolean {
	if (field.kind === 'range') {
		return !!values[field.minKey]?.trim() || !!values[field.maxKey]?.trim();
	}
	if (field.kind === 'select') {
		const v = values[field.key] ?? '';
		return v !== '' && v !== empty(field);
	}
	return !!values[field.key]?.trim();
}

export function fieldValueLabel(field: FilterField, values: FilterValues): string {
	if (!isFieldSet(field, values)) return '';
	if (field.kind === 'select') {
		const v = values[field.key];
		return field.options.find((o) => o.value === v)?.label ?? v;
	}
	if (field.kind === 'range') {
		const min = values[field.minKey]?.trim();
		const max = values[field.maxKey]?.trim();
		if (min && max) return `${min}–${max} ${field.unit}`;
		return min ? `≥ ${min} ${field.unit}` : `≤ ${max} ${field.unit}`;
	}
	return values[field.key].trim();
}

export function namedDefault(field: FilterField): string | null {
	return field.kind === 'select' && field.narrowsByDefault ? (field.anyLabel ?? null) : null;
}

// What a pill shows between the field's name and its value.
export function pillOperator(field: FilterField, values: FilterValues): string {
	if (field.kind !== 'range') return '=';
	const min = values[field.minKey]?.trim();
	const max = values[field.maxKey]?.trim();
	if (min && max) return '';
	return min ? '≥' : '≤';
}

export function pillValue(field: FilterField, values: FilterValues): string {
	if (field.kind !== 'range') return fieldValueLabel(field, values);
	const min = values[field.minKey]?.trim();
	const max = values[field.maxKey]?.trim();
	if (min && max) return `${min}–${max} ${field.unit}`;
	return `${min || max} ${field.unit}`;
}

export function isSearchField(field: FilterField): boolean {
	return field.kind === 'select' && !!field.inSearch;
}

export function clearedField(field: FilterField, values: FilterValues): FilterValues {
	if (field.kind === 'range') return { ...values, [field.minKey]: '', [field.maxKey]: '' };
	if (field.kind === 'select') return { ...values, [field.key]: empty(field) };
	return { ...values, [field.key]: '' };
}

export function clearedValues(fields: readonly FilterField[], values: FilterValues): FilterValues {
	return fields.reduce((acc, field) => clearedField(field, acc), { ...values });
}

function fieldKeys(field: FilterField): string[] {
	return field.kind === 'range' ? [field.minKey, field.maxKey] : [field.key];
}

export function valuesEqual(
	fields: readonly FilterField[],
	a: FilterValues,
	b: FilterValues
): boolean {
	return fields.every((field) =>
		fieldKeys(field).every((k) => (a[k] ?? '').trim() === (b[k] ?? '').trim())
	);
}

export function attributesEqual(
	a: readonly AttributeFilter[],
	b: readonly AttributeFilter[]
): boolean {
	return a.length === b.length && a.every((f, i) => attributeFiltersEqual(f, b[i]));
}

export function activeCount(
	fields: readonly FilterField[],
	values: FilterValues,
	attributes: readonly AttributeFilter[] = []
): number {
	return fields.filter((f) => isFieldSet(f, values)).length + attributes.length;
}

export function fieldError(field: FilterField, values: FilterValues): string | null {
	if (field.kind !== 'text' || !field.validate) return null;
	const v = values[field.key]?.trim() ?? '';
	return v ? field.validate(v) : null;
}

export function attributePill(f: AttributeFilter): string {
	return operatorOption(filterOperator(f)).pill;
}
