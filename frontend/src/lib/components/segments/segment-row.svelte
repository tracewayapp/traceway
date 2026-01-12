<script lang="ts">
	import type { Segment } from '$lib/types/segments';
	import { cn } from '$lib/utils';

	type Props = {
		segment: Segment;
		transactionStart: number;
		transactionDuration: number;
		isOdd: boolean;
	};

	let { segment, transactionStart, transactionDuration, isOdd }: Props = $props();

	const segmentStartMs = $derived(new Date(segment.startTime).getTime() - transactionStart);
	const segmentDurationMs = $derived(segment.duration / 1_000_000);
	const transactionDurationMs = $derived(transactionDuration / 1_000_000);

	// Calculate position and width as percentages
	const leftPercent = $derived(Math.max(0, (segmentStartMs / transactionDurationMs) * 100));
	const widthPercent = $derived(
		Math.min(100 - leftPercent, (segmentDurationMs / transactionDurationMs) * 100)
	);

	// 14 segment colors based on name prefix
	function getSegmentColor(name: string): { bg: string; ring: string } {
		if (name.startsWith('db.')) return { bg: 'bg-blue-400', ring: 'ring-blue-500' };
		if (name.startsWith('http.')) return { bg: 'bg-green-400', ring: 'ring-green-500' };
		if (name.startsWith('cache.')) return { bg: 'bg-purple-400', ring: 'ring-purple-500' };
		if (name.startsWith('queue.')) return { bg: 'bg-orange-400', ring: 'ring-orange-500' };
		if (name.startsWith('auth.')) return { bg: 'bg-red-400', ring: 'ring-red-500' };
		if (name.startsWith('file.')) return { bg: 'bg-amber-400', ring: 'ring-amber-500' };
		if (name.startsWith('grpc.')) return { bg: 'bg-cyan-400', ring: 'ring-cyan-500' };
		if (name.startsWith('email.')) return { bg: 'bg-pink-400', ring: 'ring-pink-500' };
		if (name.startsWith('search.')) return { bg: 'bg-indigo-400', ring: 'ring-indigo-500' };
		if (name.startsWith('redis.')) return { bg: 'bg-teal-400', ring: 'ring-teal-500' };
		if (name.startsWith('kafka.')) return { bg: 'bg-lime-400', ring: 'ring-lime-500' };
		if (name.startsWith('graphql.')) return { bg: 'bg-rose-400', ring: 'ring-rose-500' };
		if (name.startsWith('s3.')) return { bg: 'bg-sky-400', ring: 'ring-sky-500' };
		return { bg: 'bg-slate-400', ring: 'ring-slate-500' };
	}

	const segmentColor = $derived(getSegmentColor(segment.name));

	function formatDuration(nanoseconds: number): string {
		const ms = nanoseconds / 1_000_000;
		if (ms < 1) return `${(nanoseconds / 1000).toFixed(0)}µs`;
		if (ms < 1000) return `${ms.toFixed(1)}ms`;
		return `${(ms / 1000).toFixed(2)}s`;
	}

	// Tooltip state
	let isHovered = $state(false);
	let tooltipX = $state(0);
	let tooltipY = $state(0);
	let barElement: HTMLDivElement;

	function handleMouseEnter(e: MouseEvent) {
		isHovered = true;
		updateTooltipPosition(e);
	}

	function handleMouseMove(e: MouseEvent) {
		if (isHovered) {
			updateTooltipPosition(e);
		}
	}

	function handleMouseLeave() {
		isHovered = false;
	}

	function updateTooltipPosition(e: MouseEvent) {
		const rect = barElement.getBoundingClientRect();
		tooltipX = rect.left + rect.width / 2;
		tooltipY = rect.top - 8;
	}
</script>

<div class={cn('border-border flex items-center border-b last:border-b-0', isOdd ? 'bg-muted/40' : '')}>
	<!-- Segment name -->
	<div
		class="border-border w-[180px] flex-shrink-0 truncate border-r px-3 py-1.5 font-mono text-xs"
		title={segment.name}
	>
		{segment.name}
	</div>

	<!-- Timeline bar -->
	<div class="relative flex-1">
		<div class="relative mx-2 h-4">
			<div
				bind:this={barElement}
				class={cn(
					'absolute h-full rounded transition-all',
					segmentColor.bg,
					isHovered && `ring-2 ring-offset-1 ${segmentColor.ring}`
				)}
				style="left: {leftPercent}%; width: {Math.max(widthPercent, 1)}%"
				onmouseenter={handleMouseEnter}
				onmousemove={handleMouseMove}
				onmouseleave={handleMouseLeave}
				role="presentation"
			></div>
		</div>
	</div>

	<!-- Duration -->
	<div class="text-muted-foreground border-border w-[100px] flex-shrink-0 border-l px-3 py-1.5 text-right font-mono text-xs">
		{formatDuration(segment.duration)}
	</div>

	<!-- Portal tooltip (inside row to preserve :last-child, but uses fixed positioning) -->
	{#if isHovered}
		<div
			class="fixed z-[9999] -translate-x-1/2 -translate-y-full pointer-events-none"
			style="left: {tooltipX}px; top: {tooltipY}px;"
		>
			<div class="bg-popover text-popover-foreground whitespace-nowrap rounded-md border px-2 py-1 text-xs shadow-md">
				<div class="font-medium">{segment.name}</div>
				<div class="text-muted-foreground">{formatDuration(segment.duration)}</div>
			</div>
		</div>
	{/if}
</div>
