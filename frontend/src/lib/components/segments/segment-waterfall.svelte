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

<ScrollArea orientation="horizontal" class="border-border rounded-lg border p-relative">
	<div class="overflow-hidden relative">
		<!-- Header -->
		<div class="bg-muted/30 border-border flex border-b">
			<div class="border-border flex-shrink-0 border-r px-3 py-1.5 text-xs font-medium" style="min-width: {nameColumnWidth}px">
				Segment Name
			</div>
			<div bind:this={timelineElement} class="flex-1 px-3 py-1.5 min-w-[200px]">
				<div class="text-muted-foreground flex justify-between text-xs">
					<span>0ms</span>
					<span>{(durationMs / 2).toFixed(0)}ms</span>
					<span>{durationMs.toFixed(0)}ms</span>
				</div>
			</div>
			<div class="border-border w-[100px] flex-shrink-0 border-l px-3 py-1.5 text-right text-xs font-medium">
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
			<div class="top-[28px] bottom-0 border-l border-gray-300 absolute pointer-events-none" style="left: {tooltipX+nameColumnWidth}px"></div>
			<div class="top-[1px] absolute -translate-x-1/2" style="left: {tooltipX+nameColumnWidth}px">
				<div class="bg-popover text-popover-foreground whitespace-nowrap rounded-md border px-2 py-1 text-xs shadow-md">
					<div class="font-medium">{Math.round(durationMs*((tooltipX+1)/timelineElement.clientWidth))}ms</div>
				</div>
			</div>
		{/if}
	</div>
</ScrollArea>
