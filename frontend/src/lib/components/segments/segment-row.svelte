<script lang="ts">
	import type { Segment } from '$lib/types/segments';
	import { cn } from '$lib/utils';
	import { formatDuration } from '$lib/utils/formatters';

	type Props = {
		row: number;
		segment: Segment;
		transactionStart: number;
		transactionDuration: number;
		isOdd: boolean;
		nameColumnWidth: number;
		updateNameWidth: (width: number) => void;

		segmentCellHandleMouseEnter: (x: number) => void;
		segmentCellHandleMouseMove: (x: number) => void;
		segmentCellHandleMouseLeave: () => void;
	};

	let {
		row,
		segment,
		transactionStart,
		transactionDuration,
		isOdd,
		nameColumnWidth,
		updateNameWidth,
		segmentCellHandleMouseEnter,
		segmentCellHandleMouseMove,
		segmentCellHandleMouseLeave
	}: Props = $props();

	const segmentStartMs = $derived(new Date(segment.startTime).getTime() - transactionStart);
	const segmentDurationMs = $derived(segment.duration / 1_000_000);
	const transactionDurationMs = $derived(transactionDuration / 1_000_000);

	// Calculate position and width as percentages
	const leftPercent = $derived(Math.max(0, (segmentStartMs / transactionDurationMs) * 100));
	const widthPercent = $derived(
		Math.min(100 - leftPercent, (segmentDurationMs / transactionDurationMs) * 100)
	);

	const segmentColors = [
		{ bg: 'bg-blue-400', ring: 'ring-blue-500' },
		{ bg: 'bg-green-400', ring: 'ring-green-500' },
		{ bg: 'bg-purple-400', ring: 'ring-purple-500' },
		{ bg: 'bg-orange-400', ring: 'ring-orange-500' },
		{ bg: 'bg-red-400', ring: 'ring-red-500' },
		{ bg: 'bg-amber-400', ring: 'ring-amber-500' },
		{ bg: 'bg-cyan-400', ring: 'ring-cyan-500' },
		{ bg: 'bg-pink-400', ring: 'ring-pink-500' },
		{ bg: 'bg-indigo-400', ring: 'ring-indigo-500' },
		{ bg: 'bg-teal-400', ring: 'ring-teal-500' },
		{ bg: 'bg-lime-400', ring: 'ring-lime-500' },
		{ bg: 'bg-rose-400', ring: 'ring-rose-500' },
		{ bg: 'bg-sky-400', ring: 'ring-sky-500' },
		{ bg: 'bg-slate-400', ring: 'ring-slate-500' }
	];

	const segmentColor = $derived(segmentColors[row % segmentColors.length]);

	// Tooltip state (this is the tooltip on top of the line)
	let isHovered = $state(false);
	let barElement: HTMLDivElement;

	function handleMouseEnter(e: MouseEvent) {
		isHovered = true;
	}

	function handleMouseLeave() {
		isHovered = false;
	}

	let containerElement: HTMLDivElement;
	function containerSegmentCellHandleMouseEnter(e: MouseEvent) {
		const rect = containerElement.getBoundingClientRect();
		const x = e.clientX - rect.left;
		segmentCellHandleMouseEnter(x);
	}
	function containerSegmentCellHandleMouseMove(e: MouseEvent) {
		const rect = containerElement.getBoundingClientRect();
		const x = e.clientX - rect.left;
		segmentCellHandleMouseMove(x);
	}

	let nameElement: HTMLDivElement;

	$effect(() => {
		if (nameElement) {
			// Measure the natural width needed
			const naturalWidth = nameElement.scrollWidth;
			updateNameWidth?.(naturalWidth);
		}
	});
</script>

<div
	class={cn('flex items-center border-b border-border last:border-b-0', isOdd ? 'bg-muted/40' : '')}
>
	<!-- Segment name -->
	<div
		bind:this={nameElement}
		class="flex-shrink-0 truncate border-r border-border px-3 py-1.5 font-mono text-xs"
		style="min-width: {nameColumnWidth}px; max-width: {nameColumnWidth}px"
		title={segment.name}
	>
		{segment.name}
	</div>

	<!-- Timeline bar -->
	<div
		class="relative flex min-w-[200px] flex-1 items-center self-stretch"
		bind:this={containerElement}
		onmouseenter={containerSegmentCellHandleMouseEnter}
		onmousemove={containerSegmentCellHandleMouseMove}
		onmouseleave={segmentCellHandleMouseLeave}
	>
		<div class="relative h-4 w-full">
			<div
				bind:this={barElement}
				class={cn(
					'absolute h-full rounded-[2px] transition-all',
					segmentColor.bg,
					isHovered && `ring-2 ${segmentColor.ring}`
				)}
				style="left: {leftPercent}%; width: {Math.max(widthPercent, 1)}%"
				onmouseenter={handleMouseEnter}
				onmouseleave={handleMouseLeave}
				role="presentation"
			></div>
		</div>
	</div>

	<!-- Duration -->
	<div
		class="w-[100px] flex-shrink-0 border-l border-border px-3 py-1.5 text-right font-mono text-xs text-muted-foreground"
	>
		{formatDuration(segment.duration)}
	</div>
</div>
