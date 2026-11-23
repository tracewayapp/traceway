<script lang="ts" module>
	export type ChartSeriesBucket = {
		start: string;
		count: number;
		sum: number;
		avg: number;
		min: number;
		max: number;
		p50: number;
		p95: number;
		p99: number;
	};
</script>

<script lang="ts">
	import { DateTime } from 'luxon';
	import { scaleLinear } from 'd3-scale';
	import { line } from 'd3-shape';
	import {
		chartAggregations,
		formatChartNumber,
		type ChartAggregation
	} from '$lib/utils/value-chart';

	let {
		buckets,
		intervalSeconds,
		from,
		to,
		aggregations,
		timezone,
		loading = false,
		height = 180,
		format = formatChartNumber,
		countLabel = 'Values',
		onRangeSelect
	}: {
		buckets: ChartSeriesBucket[];
		intervalSeconds: number;
		from: Date;
		to: Date;
		aggregations: ChartAggregation[];
		timezone: string;
		loading?: boolean;
		height?: number;
		// How a charted value reads; counts always read as plain numbers.
		format?: (value: number) => string;
		// What the count line and the tooltip's count row call one bucket's rows.
		countLabel?: string;
		onRangeSelect?: (from: Date, to: Date) => void;
	} = $props();

	const PAD = { top: 8, right: 8, bottom: 4, left: 52 };

	let width = $state(0);
	let hoverIndex = $state<number | null>(null);
	let dragFromX = $state<number | null>(null);
	let dragToX = $state<number | null>(null);

	const plotW = $derived(Math.max(0, width - PAD.left - PAD.right));
	const plotH = $derived(Math.max(0, height - PAD.top - PAD.bottom));
	const stepMs = $derived(intervalSeconds * 1000);
	const fromMs = $derived(from.getTime());
	const toMs = $derived(Math.max(from.getTime() + 1, to.getTime()));

	const lines = $derived(chartAggregations.filter((a) => aggregations.includes(a.value)));
	const tickFormat = $derived(
		aggregations.every((a) => a === 'count') ? formatChartNumber : format
	);

	function formatLine(agg: ChartAggregation, value: number): string {
		return agg === 'count' ? value.toLocaleString() : format(value);
	}

	const points = $derived(buckets.map((b) => ({ bucket: b, t: Date.parse(b.start) + stepMs / 2 })));

	const yDomain = $derived.by((): [number, number] => {
		let lo = Infinity;
		let hi = -Infinity;
		for (const b of buckets) {
			for (const l of lines) {
				lo = Math.min(lo, b[l.value]);
				hi = Math.max(hi, b[l.value]);
			}
		}
		if (!Number.isFinite(lo)) return [0, 1];
		if (lo === hi) {
			const pad = Math.abs(lo) * 0.1 || 1;
			return [lo - pad, hi + pad];
		}
		// Counts and sums start at zero when every value is positive, like a bar.
		return [lo >= 0 && lo / hi < 0.5 ? 0 : lo, hi];
	});

	const x = $derived(scaleLinear().domain([fromMs, toMs]).range([0, plotW]));
	const y = $derived(scaleLinear().domain(yDomain).nice(4).range([plotH, 0]));
	const yTicks = $derived(y.ticks(4));

	// Buckets with no number are not returned; a line breaks where one is missing.
	const segments = $derived.by(() => {
		const out: { start: number; end: number }[] = [];
		let start = 0;
		for (let i = 1; i <= points.length; i++) {
			if (i === points.length || points[i].t - points[i - 1].t > stepMs * 1.5) {
				out.push({ start, end: i });
				start = i;
			}
		}
		return points.length === 0 ? [] : out;
	});

	function pathFor(agg: ChartAggregation, seg: { start: number; end: number }): string {
		const gen = line<(typeof points)[number]>()
			.x((p) => x(p.t))
			.y((p) => y(p.bucket[agg]));
		return gen(points.slice(seg.start, seg.end)) ?? '';
	}

	function zoned(ms: number): DateTime {
		return DateTime.fromMillis(ms).setZone(timezone);
	}

	const spanMs = $derived(toMs - fromMs);

	function axisFormat(ms: number): string {
		const dt = zoned(ms);
		if (spanMs > 36 * 3600_000) return dt.toLocaleString({ month: 'short', day: 'numeric' });
		if (spanMs > 12 * 3600_000)
			return dt.toLocaleString({
				month: 'short',
				day: 'numeric',
				hour: 'numeric',
				minute: '2-digit'
			});
		if (stepMs < 60_000) return dt.toLocaleString(DateTime.TIME_WITH_SECONDS);
		return dt.toLocaleString(DateTime.TIME_SIMPLE);
	}

	const axisLabels = $derived.by(() => {
		const n = width < 560 ? 3 : 5;
		return Array.from({ length: n }, (_, i) => axisFormat(fromMs + (spanMs * i) / (n - 1)));
	});

	function rangeLabel(bucket: ChartSeriesBucket): string {
		const start = zoned(Date.parse(bucket.start));
		const end = start.plus({ milliseconds: stepMs });
		const time = stepMs < 60_000 ? DateTime.TIME_WITH_SECONDS : DateTime.TIME_SIMPLE;
		if (stepMs >= 86400_000) return start.toLocaleString(DateTime.DATE_MED);
		return `${start.toLocaleString(DateTime.DATE_MED)}, ${start.toLocaleString(time)} – ${end.toLocaleString(time)}`;
	}

	function plotX(e: PointerEvent): number {
		const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
		return Math.max(0, Math.min(plotW, e.clientX - rect.left - PAD.left));
	}

	function nearest(px: number): number | null {
		if (points.length === 0) return null;
		const t = x.invert(px);
		let best = 0;
		for (let i = 1; i < points.length; i++) {
			if (Math.abs(points[i].t - t) < Math.abs(points[best].t - t)) best = i;
		}
		return Math.abs(x(points[best].t) - px) <= Math.max(24, x(fromMs + stepMs) - x(fromMs))
			? best
			: null;
	}

	function handlePointerMove(e: PointerEvent) {
		const px = plotX(e);
		hoverIndex = nearest(px);
		if (dragFromX !== null) dragToX = px;
	}

	function handlePointerDown(e: PointerEvent) {
		if (!onRangeSelect || e.button !== 0) return;
		dragFromX = plotX(e);
		dragToX = dragFromX;
		(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
		e.preventDefault();
	}

	function handlePointerUp() {
		if (dragFromX === null || dragToX === null) return;
		const lo = Math.min(dragFromX, dragToX);
		const hi = Math.max(dragFromX, dragToX);
		dragFromX = null;
		dragToX = null;
		if (hi - lo < 6) return;
		onRangeSelect?.(new Date(x.invert(lo)), new Date(x.invert(hi)));
	}

	$effect(() => {
		if (dragFromX === null) return;
		const onKey = (e: KeyboardEvent) => {
			if (e.key === 'Escape') {
				dragFromX = null;
				dragToX = null;
			}
		};
		window.addEventListener('keydown', onKey);
		return () => window.removeEventListener('keydown', onKey);
	});

	const hovered = $derived(hoverIndex !== null ? points[hoverIndex] : null);
	const tooltipLeft = $derived(hovered ? PAD.left + x(hovered.t) : 0);
</script>

<div class="flex flex-col gap-1">
	<div
		class="relative touch-none select-none {onRangeSelect ? 'cursor-crosshair' : ''}"
		style:height="{height}px"
		bind:clientWidth={width}
		role="application"
		aria-label="The charted values over the selected time range. Drag across the chart to zoom in."
		onpointermove={handlePointerMove}
		onpointerleave={() => (hoverIndex = null)}
		onpointerdown={handlePointerDown}
		onpointerup={handlePointerUp}
		onpointercancel={() => {
			dragFromX = null;
			dragToX = null;
		}}
	>
		{#if width > 0}
			<svg {width} {height} class="block transition-opacity {loading ? 'opacity-50' : ''}">
				<g transform="translate({PAD.left}, {PAD.top})">
					{#each yTicks as tick (tick)}
						<line
							x1={0}
							x2={plotW}
							y1={y(tick)}
							y2={y(tick)}
							stroke="currentColor"
							class="text-border"
							stroke-dasharray={tick === 0 ? '' : '3 3'}
						/>
						<text
							x={-8}
							y={y(tick)}
							text-anchor="end"
							dominant-baseline="middle"
							font-size="10"
							fill="currentColor"
							class="text-muted-foreground tabular-nums"
						>
							{tickFormat(tick)}
						</text>
					{/each}
					{#if dragFromX !== null && dragToX !== null}
						<rect
							x={Math.min(dragFromX, dragToX)}
							y={0}
							width={Math.abs(dragToX - dragFromX)}
							height={plotH}
							fill="var(--primary)"
							fill-opacity="0.15"
							rx="3"
						/>
					{/if}
					{#if hovered && dragFromX === null}
						<line
							x1={x(hovered.t)}
							x2={x(hovered.t)}
							y1={0}
							y2={plotH}
							stroke="currentColor"
							class="text-muted-foreground"
							stroke-opacity="0.5"
						/>
					{/if}
					{#each lines as l (l.value)}
						{#each segments as seg, i (i)}
							{#if seg.end - seg.start > 1}
								<path
									d={pathFor(l.value, seg)}
									fill="none"
									stroke={l.color}
									stroke-width="2"
									stroke-linejoin="round"
									stroke-linecap="round"
								/>
							{:else}
								<circle
									cx={x(points[seg.start].t)}
									cy={y(points[seg.start].bucket[l.value])}
									r="2.5"
									fill={l.color}
								/>
							{/if}
						{/each}
						{#if hovered && dragFromX === null}
							<circle
								cx={x(hovered.t)}
								cy={y(hovered.bucket[l.value])}
								r="3.5"
								fill={l.color}
								stroke="var(--background)"
								stroke-width="1.5"
							/>
						{/if}
					{/each}
				</g>
			</svg>
		{/if}

		{#if hovered && dragFromX === null}
			<div
				class="pointer-events-none absolute top-2 z-20 min-w-44 rounded-md border border-border bg-popover px-2.5 py-1.5 text-xs text-popover-foreground shadow-md"
				style="left: {tooltipLeft}px; transform: translateX({tooltipLeft > width - 200
					? 'calc(-100% - 12px)'
					: '12px'});"
			>
				<div class="mb-1 whitespace-nowrap text-muted-foreground">{rangeLabel(hovered.bucket)}</div>
				{#each lines as l (l.value)}
					<div class="flex items-center gap-1.5 whitespace-nowrap">
						<span class="h-2 w-2 rounded-full" style="background-color: {l.color};"></span>
						<span class="text-muted-foreground">{l.label}</span>
						<span class="ml-auto pl-3 font-medium tabular-nums">
							{formatLine(l.value, hovered.bucket[l.value])}
						</span>
					</div>
				{/each}
				{#if !aggregations.includes('count')}
					<div class="mt-1 flex justify-between border-t border-border pt-1">
						<span class="text-muted-foreground">{countLabel}</span>
						<span class="font-medium tabular-nums">{hovered.bucket.count.toLocaleString()}</span>
					</div>
				{/if}
			</div>
		{/if}
	</div>
	<div
		class="flex justify-between text-[10px] leading-none text-muted-foreground tabular-nums"
		style:padding-left="{PAD.left}px"
		style:padding-right="{PAD.right}px"
	>
		{#each axisLabels as label, i (i)}
			<span>{label}</span>
		{/each}
	</div>
</div>
