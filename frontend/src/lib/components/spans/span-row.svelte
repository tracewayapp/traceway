<script lang="ts">
	import type { Span } from '$lib/types/spans';
	import { cn } from '$lib/utils';
	import { formatDuration } from '$lib/utils/formatters';
	import * as Popover from '$lib/components/ui/popover';
	import Copy from 'lucide-svelte/icons/copy';
	import Check from 'lucide-svelte/icons/check';

	type Props = {
		row: number;
		span: Span;
		traceStart: number;
		traceDuration: number;
		isOdd: boolean;
		nameColumnWidth: number;
		updateNameWidth: (width: number) => void;

		spanCellHandleMouseEnter: (x: number) => void;
		spanCellHandleMouseMove: (x: number) => void;
		spanCellHandleMouseLeave: () => void;
	};

	let {
		row,
		span,
		traceStart,
		traceDuration,
		isOdd,
		nameColumnWidth,
		updateNameWidth,
		spanCellHandleMouseEnter,
		spanCellHandleMouseMove,
		spanCellHandleMouseLeave
	}: Props = $props();

	const spanStartMs = $derived(new Date(span.startTime).getTime() - traceStart);
	const spanDurationMs = $derived(span.duration / 1_000_000);
	const traceDurationMs = $derived(traceDuration / 1_000_000);

	// Calculate position and width as percentages
	const leftPercent = $derived(Math.max(0, (spanStartMs / traceDurationMs) * 100));
	const widthPercent = $derived(
		Math.min(100 - leftPercent, (spanDurationMs / traceDurationMs) * 100)
	);

	const spanColors = [
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

	const spanColor = $derived(spanColors[row % spanColors.length]);

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
	function containerSpanCellHandleMouseEnter(e: MouseEvent) {
		const rect = containerElement.getBoundingClientRect();
		const x = e.clientX - rect.left;
		spanCellHandleMouseEnter(x);
	}
	function containerSpanCellHandleMouseMove(e: MouseEvent) {
		const rect = containerElement.getBoundingClientRect();
		const x = e.clientX - rect.left;
		spanCellHandleMouseMove(x);
	}

	let nameElement: HTMLDivElement;
	let copied = $state(false);

	async function copySpanName() {
		await navigator.clipboard.writeText(span.name);
		copied = true;
		setTimeout(() => (copied = false), 2000);
	}

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
	<!-- Span name -->
	<Popover.Root>
		<Popover.Trigger class="text-left cursor-pointer">
			<div
				bind:this={nameElement}
				class="flex-shrink-0 truncate border-r border-border px-3 py-1.5 font-mono text-xs"
				style="min-width: {nameColumnWidth}px; max-width: {nameColumnWidth}px"
			>
				{span.name}
			</div>
		</Popover.Trigger>
		<Popover.Content class="w-auto max-w-sm" align="start">
			<div class="flex items-start gap-2">
				<span class="font-mono text-xs break-all select-text">{span.name}</span>
				<button onclick={copySpanName} class="shrink-0 p-1 rounded hover:bg-muted">
					{#if copied}
						<Check class="h-3.5 w-3.5 text-green-500" />
					{:else}
						<Copy class="h-3.5 w-3.5 text-muted-foreground" />
					{/if}
				</button>
			</div>
		</Popover.Content>
	</Popover.Root>

	<!-- Timeline bar -->
	<div
		class="relative flex min-w-[200px] flex-1 items-center self-stretch"
		bind:this={containerElement}
		onmouseenter={containerSpanCellHandleMouseEnter}
		onmousemove={containerSpanCellHandleMouseMove}
		onmouseleave={spanCellHandleMouseLeave}
	>
		<div class="relative h-4 w-full">
			<div
				bind:this={barElement}
				class={cn(
					'absolute h-full rounded-[2px] transition-all',
					spanColor.bg,
					isHovered && `ring-2 ${spanColor.ring}`
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
		{formatDuration(span.duration)}
	</div>
</div>
