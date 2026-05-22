<script lang="ts">
	import type { Span, ChildEntity } from '$lib/types/spans';
	import ScrollArea from '../ui/scroll-area/scroll-area.svelte';
	import SpanRow from './span-row.svelte';
	import ChildEntityRow from './child-entity-row.svelte';
	import { preciseTimeMs } from '$lib/utils/formatters';

	type Props = {
		spans: Span[];
		traceDuration: number;
		traceStartTime: string;
		childEntities?: ChildEntity[];
	};

	let { spans: rawSpans, traceDuration, traceStartTime, childEntities = [] }: Props = $props();

	// Tagged union so we can interleave spans + child entities in one sorted list.
	type Row =
		| { type: 'span'; startMs: number; span: Span }
		| { type: 'child'; startMs: number; entity: ChildEntity };

	const rows = $derived.by<Row[]>(() => {
		const merged: Row[] = [];
		for (const s of rawSpans) {
			merged.push({ type: 'span', startMs: preciseTimeMs(s.startTime), span: s });
		}
		for (const ce of childEntities) {
			merged.push({ type: 'child', startMs: preciseTimeMs(ce.recordedAt), entity: ce });
		}
		merged.sort((a, b) => a.startMs - b.startMs);
		return merged;
	});

	const traceStart = $derived(
		rows.length === 0
			? preciseTimeMs(traceStartTime)
			: rows.reduce((earliest, r) => (r.startMs < earliest ? r.startMs : earliest), rows[0].startMs)
	);
	const durationMs = $derived(traceDuration / 1_000_000);

	let nameColumnWidth = $state(180); // default minimum

	function updateNameWidth(width: number) {
		if (width > 400) {
			width = 400;
		}
		if (width > nameColumnWidth) {
			nameColumnWidth = width;
		}
	}

	let isHovered = $state(false);
	let tooltipX = $state(0);

	function handleMouseEnter(x: number) {
		isHovered = true;
		tooltipX = x;
	}

	function handleMouseMove(x: number) {
		if (isHovered) {
			tooltipX = x;
		}
	}

	function handleMouseLeave() {
		isHovered = false;
	}

	let timelineElement: HTMLDivElement;
</script>

<ScrollArea orientation="horizontal" class="p-relative rounded-lg border border-border">
	<div class="relative overflow-hidden">
		<!-- Header -->
		<div class="flex border-b border-border bg-muted/30">
			<div
				class="flex-shrink-0 border-r border-border px-3 py-1.5 text-xs font-medium"
				style="min-width: {nameColumnWidth}px"
			>
				Span Name
			</div>
			<div bind:this={timelineElement} class="min-w-[200px] flex-1 px-3 py-1.5">
				<div class="flex justify-between text-xs text-muted-foreground">
					<span>0ms</span>
					<span>{(durationMs / 2).toFixed(0)}ms</span>
					<span>{durationMs.toFixed(0)}ms</span>
				</div>
			</div>
			<div
				class="w-[100px] flex-shrink-0 border-l border-border px-3 py-1.5 text-right text-xs font-medium"
			>
				Duration
			</div>
		</div>

		<!-- Spans + child entities, interleaved by start time -->
		{#each rows as row, i}
			{#if row.type === 'span'}
				<SpanRow
					row={i}
					span={row.span}
					{traceStart}
					{traceDuration}
					isOdd={i % 2 === 1}
					{nameColumnWidth}
					{updateNameWidth}
					spanCellHandleMouseEnter={handleMouseEnter}
					spanCellHandleMouseMove={handleMouseMove}
					spanCellHandleMouseLeave={handleMouseLeave}
				/>
			{:else}
				<ChildEntityRow
					row={i}
					entity={row.entity}
					{traceStart}
					{traceDuration}
					isOdd={i % 2 === 1}
					{nameColumnWidth}
				/>
			{/if}
		{/each}

		{#if isHovered}
			<div
				class="pointer-events-none absolute top-[28px] bottom-0 border-l border-gray-300"
				style="left: {tooltipX + nameColumnWidth}px"
			></div>
			<div class="absolute top-[1px] -translate-x-1/2" style="left: {tooltipX + nameColumnWidth}px">
				<div
					class="rounded-md border bg-popover px-2 py-1 text-xs whitespace-nowrap text-popover-foreground shadow-md"
				>
					<div class="font-medium">
						{Math.round(durationMs * ((tooltipX + 1) / (timelineElement?.clientWidth || 1)))}ms
					</div>
				</div>
			</div>
		{/if}
	</div>
</ScrollArea>
