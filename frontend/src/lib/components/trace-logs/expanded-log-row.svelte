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
	import type { FilterOperator } from '$lib/utils/attribute-filters';

	let {
		log,
		colspan,
		filterOperatorFor,
		onFilterOperator,
		isMetaFilterActive,
		onMetaFilterToggle,
		onHeightChange,
		showAllAttributes = $bindable(false)
	}: {
		log: ExpandedLogRecord;
		colspan: number;
		filterOperatorFor?: (key: string, value: string) => FilterOperator | null;
		onFilterOperator?: (key: string, value: string, operator: FilterOperator | null) => void;
		isMetaFilterActive?: (field: MetaFilterField, value: string) => boolean;
		onMetaFilterToggle?: (field: MetaFilterField, value: string) => void;
		onHeightChange?: (height: number) => void;
		// Bind it to keep "Show more" open across remounts: the row is rebuilt
		// whenever the list reloads (every filter change) or scrolls back in.
		showAllAttributes?: boolean;
	} = $props();

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

	function mergedAttributes(): Record<string, string> {
		const out: Record<string, string> = {};
		const push = (m: Record<string, string> | null | undefined, prefix: string) => {
			if (!m) return;
			for (const [k, v] of Object.entries(m)) {
				out[`${prefix}${k}`] = v;
			}
		};
		push(log.resourceAttributes, 'resource.');
		push(log.scopeAttributes, 'scope.');
		push(log.logAttributes, '');
		return out;
	}

	function hasAttributes(): boolean {
		for (const _ in mergedAttributes()) return true;
		return false;
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
			{#if hasAttributes()}
				<AttributesGrid
					attributes={mergedAttributes()}
					collapsedCount={6}
					{filterOperatorFor}
					{onFilterOperator}
					bind:expanded={showAllAttributes}
				/>
			{/if}
		</div>
	</Table.Cell>
</Table.Row>
