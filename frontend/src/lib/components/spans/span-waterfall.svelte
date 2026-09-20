<script lang="ts">
	import type { Span, SpanAttributeLoader } from '$lib/types/spans';
	import { SvelteSet } from 'svelte/reactivity';
	import ScrollArea from '../ui/scroll-area/scroll-area.svelte';
	import SpanRow from './span-row.svelte';
	import { preciseTimeMs } from '$lib/utils/formatters';
	import { buildSpanTree, flattenBuiltTree, initiallyCollapsed } from '$lib/utils/span-tree';
	import ChevronsDownUp from '@lucide/svelte/icons/chevrons-down-up';
	import ChevronsUpDown from '@lucide/svelte/icons/chevrons-up-down';
	import { spanServices } from '$lib/utils/span-fields';
	import { spanTraceHref } from '$lib/utils/span-explorer';
	import { resolveHref } from '$lib/utils/links';

	type Props = {
		spans: Span[];
		traceDuration: number;
		traceStartTime: string;
		// The trace the waterfall belongs to, for the link to the whole trace. Without it the first span names the trace.
		traceId?: string;
		rootSpanId?: string;
		linkTraces?: boolean;
		selectedSpanId?: string | null;
		// Set by a page whose spans arrive without attributes: the popover asks for them when it opens.
		loadAttributes?: SpanAttributeLoader;
		missingParentLabel?: string;
		// Set by the whole trace view, whose spans come from every project of the organization.
		projectNames?: Record<string, string>;
	};

	let {
		spans: rawSpans,
		traceDuration,
		traceStartTime,
		traceId,
		rootSpanId,
		linkTraces = true,
		selectedSpanId = null,
		loadAttributes,
		missingParentLabel = 'Parent span is unavailable',
		projectNames
	}: Props = $props();

	const services = $derived(spanServices(rawSpans));
	const showService = $derived(services.length > 1);

	// One colour per service once a trace crosses services, so the hops read at a glance. A single service keeps a colour per row.
	function colorIndex(service: string | undefined, row: number): number {
		return showService ? Math.max(0, services.indexOf(service ?? '')) : row;
	}

	// A complete trace can hold thousands of spans. Rendering them all at once stalls the page.
	const ROW_PAGE = 500;
	let visibleRows = $state(ROW_PAGE);

	const wholeTraceHref = $derived.by(() => {
		if (!linkTraces) return undefined;
		if (traceId) return spanTraceHref({ traceId, spanId: '', startTime: traceStartTime });
		return rawSpans.length ? spanTraceHref(rawSpans[0]) : undefined;
	});

	const tree = $derived(buildSpanTree(rawSpans, rootSpanId, !!projectNames));

	// A large trace opens with its widest nodes folded. What someone clicks is kept as the difference from that default,
	// so the default can follow the spans without an effect.
	const foldedByDefault = $derived(initiallyCollapsed(tree));
	const flipped = new SvelteSet<string>();
	const collapsed = $derived(
		new Set([
			...[...foldedByDefault].filter((key) => !flipped.has(key)),
			...[...flipped].filter((key) => !foldedByDefault.has(key))
		])
	);

	function toggleCollapse(key: string) {
		if (!flipped.delete(key)) flipped.add(key);
	}

	function flipExactly(keys: Iterable<string>) {
		flipped.clear();
		for (const key of keys) flipped.add(key);
		visibleRows = ROW_PAGE;
	}

	const expandAll = () => flipExactly(foldedByDefault);
	const collapseAll = () =>
		flipExactly([...tree.childrenById.keys()].filter((key) => !foldedByDefault.has(key)));

	const treeRows = $derived(flattenBuiltTree(tree, collapsed));

	const traceStart = $derived(
		rawSpans.reduce(
			(earliest, span) => Math.min(earliest, preciseTimeMs(span.startTime)),
			preciseTimeMs(traceStartTime)
		)
	);
	const timelineDuration = $derived(
		Math.max(
			traceDuration,
			...rawSpans.map(
				(span) => (preciseTimeMs(span.startTime) - traceStart) * 1_000_000 + span.duration
			),
			(preciseTimeMs(traceStartTime) - traceStart) * 1_000_000 + traceDuration,
			1
		)
	);
	const durationMs = $derived(timelineDuration / 1_000_000);

	let measuredNameWidth = $state(180); // default minimum
	let containerWidth = $state(0);

	// On a phone a 400px name column would push every bar off screen.
	const nameColumnWidth = $derived(
		containerWidth > 0
			? Math.min(measuredNameWidth, Math.max(140, Math.round(containerWidth * 0.5)))
			: measuredNameWidth
	);

	// On a narrow screen the prefix would crowd out the span name. The bar colour and the popover still name the service.
	const showServiceLabel = $derived(showService && (containerWidth === 0 || containerWidth >= 520));

	function updateNameWidth(width: number) {
		if (width > 400) {
			width = 400;
		}
		if (width > measuredNameWidth) {
			measuredNameWidth = width;
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

	let timelineElement = $state<HTMLDivElement>();
</script>

<div bind:clientWidth={containerWidth}>
	<ScrollArea orientation="horizontal" class="p-relative rounded-md border border-border">
		<div class="relative overflow-hidden">
			<!-- Header -->
			<div class="flex border-b border-border bg-muted/30">
				<div
					class="flex-shrink-0 border-r border-border px-3 py-1.5 text-xs font-medium"
					style="min-width: {nameColumnWidth}px"
				>
					<div class="flex items-center justify-between gap-2">
						<span>Span Name</span>
						{#if tree.childrenById.size > 0}
							<span class="flex items-center gap-0.5">
								<button
									type="button"
									class="cursor-pointer rounded p-0.5 text-muted-foreground hover:bg-muted hover:text-foreground"
									aria-label="Expand all"
									title="Expand all"
									onclick={expandAll}
								>
									<ChevronsUpDown class="h-3.5 w-3.5" />
								</button>
								<button
									type="button"
									class="cursor-pointer rounded p-0.5 text-muted-foreground hover:bg-muted hover:text-foreground"
									aria-label="Collapse all"
									title="Collapse all"
									onclick={collapseAll}
								>
									<ChevronsDownUp class="h-3.5 w-3.5" />
								</button>
							</span>
						{/if}
					</div>
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

			<!-- Spans -->
			{#each treeRows.slice(0, visibleRows) as treeRow, i (treeRow.key)}
				<SpanRow
					row={colorIndex(treeRow.span.serviceName, i)}
					span={treeRow.span}
					{traceStart}
					traceDuration={timelineDuration}
					isOdd={i % 2 === 1}
					depth={treeRow.depth}
					hasChildren={treeRow.hasChildren}
					missingParent={treeRow.missingParent}
					{missingParentLabel}
					descendants={treeRow.descendants}
					errorBelow={treeRow.errorBelow}
					{loadAttributes}
					projectName={projectNames?.[treeRow.span.projectId]}
					showService={showServiceLabel}
					isSelected={!!selectedSpanId && treeRow.span.spanId === selectedSpanId}
					traceHref={linkTraces ? spanTraceHref(treeRow.span) : undefined}
					isCollapsed={collapsed.has(treeRow.key)}
					onToggle={() => toggleCollapse(treeRow.key)}
					{nameColumnWidth}
					{updateNameWidth}
					spanCellHandleMouseEnter={handleMouseEnter}
					spanCellHandleMouseMove={handleMouseMove}
					spanCellHandleMouseLeave={handleMouseLeave}
				/>
			{/each}

			{#if treeRows.length > visibleRows || wholeTraceHref}
				<div
					class="flex flex-wrap items-center justify-between gap-2 border-t border-border bg-muted/30 px-3 py-1.5 text-xs text-muted-foreground"
				>
					{#if treeRows.length > visibleRows}
						<button
							type="button"
							class="cursor-pointer underline underline-offset-2 hover:text-foreground"
							onclick={() => (visibleRows += ROW_PAGE)}
						>
							Show {Math.min(ROW_PAGE, treeRows.length - visibleRows)} more of {treeRows.length -
								visibleRows} hidden spans
						</button>
					{:else}
						<span></span>
					{/if}
					{#if wholeTraceHref}
						<a
							class="underline underline-offset-2 hover:text-foreground"
							{...{ href: resolveHref(wholeTraceHref) }}>Open the whole trace</a
						>
					{/if}
				</div>
			{/if}

			{#if isHovered}
				<div
					class="pointer-events-none absolute top-[28px] bottom-0 border-l border-border"
					style="left: {tooltipX + nameColumnWidth}px"
				></div>
				<div
					class="absolute top-[1px] -translate-x-1/2"
					style="left: {tooltipX + nameColumnWidth}px"
				>
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
</div>
