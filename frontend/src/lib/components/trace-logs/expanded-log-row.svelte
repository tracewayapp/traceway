<script lang="ts" module>
	export type ExpandedLogRecord = {
		body: string;
		traceId?: string;
		spanId?: string;
		scopeName?: string;
		scopeVersion?: string;
		resourceAttributes?: Record<string, string> | null;
		scopeAttributes?: Record<string, string> | null;
		logAttributes?: Record<string, string> | null;
	};

	export type MetaFilterField = 'body' | 'traceId' | 'spanId' | 'scopeName';
</script>

<script lang="ts">
	import * as Table from '$lib/components/ui/table';
	import { AttributesGrid, AttributesView } from '$lib/components/ui/attributes-grid';
	import { logOperatorOptions, type FilterOperator } from '$lib/utils/attribute-filters';
	import { isNumericAttributeValue } from '$lib/utils/log-attribute-chart';
	import { logAttributeEntries, type LogAttributeTarget } from '$lib/utils/log-filters';

	let {
		log,
		colspan,
		filterOperatorFor,
		onFilterOperator,
		onChart,
		isMetaFilterActive,
		onMetaFilterToggle,
		onHeightChange,
		showAllAttributes = $bindable(false)
	}: {
		log: ExpandedLogRecord;
		colspan: number;
		filterOperatorFor?: (target: LogAttributeTarget, value: string) => FilterOperator | null;
		onFilterOperator?: (
			target: LogAttributeTarget,
			value: string,
			operator: FilterOperator | null
		) => void;
		// Offered on attributes whose value is a number.
		onChart?: (target: LogAttributeTarget) => void;
		isMetaFilterActive?: (field: MetaFilterField, value: string) => boolean;
		onMetaFilterToggle?: (field: MetaFilterField, value: string) => void;
		onHeightChange?: (height: number) => void;
		// Bind it to keep "Show more" open across remounts: the row is rebuilt
		// whenever the list reloads (every filter change) or scrolls back in.
		showAllAttributes?: boolean;
	} = $props();

	// The grid works with the keys it shows; the page with the layer and key.
	const entries = $derived(logAttributeEntries(log));

	let row = $state<HTMLTableRowElement | null>(null);
	let contentHeight = $state(0);

	// The row grows and shrinks after mount (Show more, long values, JSON
	// trees). The content block is what gets measured because browsers do not
	// all report size changes of table-row boxes.
	$effect(() => {
		if (contentHeight > 0 && row) onHeightChange?.(row.offsetHeight);
	});

	// Body, trace id, span id and scope match exactly, so their tiles toggle an
	// equals filter in one click instead of offering the operator menu.
	function metaTileProps(field: MetaFilterField, value: string) {
		return {
			activeOperator: isMetaFilterActive?.(field, value) ? ('=' as const) : null,
			onFilterToggle: onMetaFilterToggle ? () => onMetaFilterToggle(field, value) : undefined
		};
	}

	function targetOf(displayKey: string): LogAttributeTarget {
		return entries.targets.get(displayKey) ?? { scope: 'log', key: displayKey };
	}
</script>

<!--
	The container <tr>/<td> carry a fixed background with `!` so Table.Row's
	built-in hover:[&>td]:bg-muted/50 can't repaint the cell on mouseover.
-->
<Table.Row bind:ref={row} class="!bg-background">
	<Table.Cell {colspan} class="!bg-background px-4 py-3">
		<div bind:offsetHeight={contentHeight} class="space-y-3">
			<div class="grid grid-cols-1 gap-3 sm:grid-cols-2 md:grid-cols-3">
				<div class="sm:col-span-2 md:col-span-3">
					<AttributesView title="Body" value={log.body} {...metaTileProps('body', log.body)} />
				</div>
				{#if log.traceId}
					<AttributesView
						title="Trace ID"
						value={log.traceId}
						{...metaTileProps('traceId', log.traceId)}
					/>
				{/if}
				{#if log.spanId}
					<AttributesView
						title="Span ID"
						value={log.spanId}
						{...metaTileProps('spanId', log.spanId)}
					/>
				{/if}
				{#if log.scopeName}
					<!-- The tile shows name@version, but the filter matches scope_name. -->
					<AttributesView
						title="Scope"
						value={log.scopeName + (log.scopeVersion ? `@${log.scopeVersion}` : '')}
						{...metaTileProps('scopeName', log.scopeName)}
					/>
				{/if}
			</div>
			{#if entries.targets.size > 0}
				<AttributesGrid
					attributes={entries.attributes}
					collapsedCount={6}
					operators={logOperatorOptions}
					filterOperatorFor={filterOperatorFor
						? (key, value) => filterOperatorFor(targetOf(key), value)
						: undefined}
					onFilterOperator={onFilterOperator
						? (key, value, operator) => onFilterOperator(targetOf(key), value, operator)
						: undefined}
					canChart={(_, value) => isNumericAttributeValue(value)}
					onChart={onChart ? (key) => onChart(targetOf(key)) : undefined}
					bind:expanded={showAllAttributes}
				/>
			{/if}
		</div>
	</Table.Cell>
</Table.Row>
