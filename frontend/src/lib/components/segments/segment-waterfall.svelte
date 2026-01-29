<script lang="ts">
	import type { Segment } from '$lib/types/segments';
	import ScrollArea from '../ui/scroll-area/scroll-area.svelte';
	import SegmentRow from './segment-row.svelte';

	type Props = {
		segments: Segment[];
		transactionDuration: number;
		transactionStartTime: string;
	};

	let { segments, transactionDuration, transactionStartTime }: Props = $props();

	const transactionStart = $derived(
		segments.length === 0
			? new Date(transactionStartTime).getTime()
			: segments.reduce((earliest, seg) => {
					const segTime = new Date(seg.startTime).getTime();
					return segTime < earliest ? segTime : earliest;
				}, new Date(segments[0].startTime).getTime())
	);
	const durationMs = $derived(transactionDuration / 1_000_000);

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
				Segment Name
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

		<!-- Segments -->
		{#each segments as segment, i}
			<SegmentRow
				row={i}
				{segment}
				{transactionStart}
				{transactionDuration}
				isOdd={i % 2 === 1}
				{nameColumnWidth}
				{updateNameWidth}
				segmentCellHandleMouseEnter={handleMouseEnter}
				segmentCellHandleMouseMove={handleMouseMove}
				segmentCellHandleMouseLeave={handleMouseLeave}
			/>
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
