<script lang="ts">
	import type { Segment } from '$lib/types/segments';
	import SegmentRow from './segment-row.svelte';

	type Props = {
		segments: Segment[];
		transactionDuration: number;
		transactionStartTime: string;
	};

	let { segments, transactionDuration, transactionStartTime }: Props = $props();

	// Calculate the time scale for the waterfall
	// Use the earliest segment's start time as reference since transaction.recordedAt may be truncated
	const transactionStart = $derived(
		segments.length === 0
			? new Date(transactionStartTime).getTime()
			: segments.reduce((earliest, seg) => {
					const segTime = new Date(seg.startTime).getTime();
					return segTime < earliest ? segTime : earliest;
				}, new Date(segments[0].startTime).getTime())
	);
	const durationMs = $derived(transactionDuration / 1_000_000);
</script>

<div class="border-border overflow-hidden rounded-lg border">
	<!-- Header -->
	<div class="bg-muted/30 border-border flex border-b">
		<div class="border-border w-[180px] flex-shrink-0 border-r px-3 py-1.5 text-xs font-medium">
			Segment Name
		</div>
		<div class="flex-1 px-3 py-1.5">
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
		<SegmentRow {segment} {transactionStart} {transactionDuration} isOdd={i % 2 === 1} />
	{/each}
</div>
