<script lang="ts" module>
	export type LogVolumeBucket = {
		start: string;
		fatal: number;
		error: number;
		warn: number;
		info: number;
		debug: number;
		trace: number;
		unknown: number;
	};

	export type LogVolumeLevel = Exclude<keyof LogVolumeBucket, 'start'>;

	// Most severe first: the order of the tooltip and legend, and of the stack from
	// the baseline up, so errors sit on the axis where they are easiest to compare.
	export const LOG_VOLUME_LEVELS: { key: LogVolumeLevel; label: string; color: string }[] = [
		{ key: 'fatal', label: 'Fatal', color: 'var(--log-fatal)' },
		{ key: 'error', label: 'Error', color: 'var(--log-error)' },
		{ key: 'warn', label: 'Warn', color: 'var(--log-warn)' },
		{ key: 'info', label: 'Info', color: 'var(--log-info)' },
		{ key: 'debug', label: 'Debug', color: 'var(--log-debug)' },
		{ key: 'trace', label: 'Trace', color: 'var(--log-trace)' },
		{ key: 'unknown', label: 'No level', color: 'var(--log-unknown)' }
	];

	export function bucketTotal(b: LogVolumeBucket): number {
		return b.fatal + b.error + b.warn + b.info + b.debug + b.trace + b.unknown;
	}
</script>

<script lang="ts">
	import { DateTime } from 'luxon';

	let {
		buckets,
		intervalSeconds,
		total,
		timezone,
		loading = false,
		height = 40,
		onRangeSelect
	}: {
		buckets: LogVolumeBucket[];
		intervalSeconds: number;
		total: number;
		timezone: string;
		loading?: boolean;
		height?: number;
		onRangeSelect?: (from: Date, to: Date) => void;
	} = $props();

	const BAR_WIDTH_RATIO = 0.8;
	const BAR_OPACITY = 0.85;
	const RADIUS = 2;
	const EMPTY_HEIGHT = 2;
	const MIN_BAR_HEIGHT = 2;

	let width = $state(0);
	let hoverIndex = $state<number | null>(null);
	let dragFrom = $state<number | null>(null);
	let dragTo = $state<number | null>(null);

	const slot = $derived(buckets.length > 0 ? width / buckets.length : 0);
	const barWidth = $derived(Math.max(1, Math.min(slot - 1, slot * BAR_WIDTH_RATIO)));
	const maxTotal = $derived(Math.max(1, ...buckets.map(bucketTotal)));
	const stepMs = $derived(intervalSeconds * 1000);

	const levelTotals = $derived(
		LOG_VOLUME_LEVELS.map((level) => ({
			...level,
			count: buckets.reduce((sum, b) => sum + b[level.key], 0)
		})).filter((level) => level.count > 0)
	);

	type Segment = { y: number; h: number; color: string; top: boolean };

	const bars = $derived(
		buckets.map((b, i) => {
			const x = i * slot + (slot - barWidth) / 2;
			const sum = bucketTotal(b);
			if (sum === 0) return { x, segments: [] as Segment[] };
			const barHeight = Math.max(MIN_BAR_HEIGHT, (sum / maxTotal) * height);
			const segments: Segment[] = [];
			let y = height;
			for (const level of LOG_VOLUME_LEVELS) {
				const count = b[level.key];
				if (count === 0) continue;
				const h = (count / sum) * barHeight;
				y -= h;
				segments.push({ y, h, color: level.color, top: false });
			}
			segments[segments.length - 1].top = true;
			return { x, segments };
		})
	);

	// A rectangle whose top corners are rounded, so the bar reads as one mark however
	// many levels it stacks.
	function roundedTopPath(x: number, y: number, w: number, h: number): string {
		const r = Math.min(RADIUS, w / 2, h);
		return `M${x},${y + h}V${y + r}Q${x},${y} ${x + r},${y}H${x + w - r}Q${x + w},${y} ${x + w},${y + r}V${y + h}Z`;
	}

	const spanMs = $derived(buckets.length > 0 ? buckets.length * stepMs : 0);
	const firstStart = $derived(buckets.length > 0 ? Date.parse(buckets[0].start) : 0);

	function zoned(ms: number): DateTime {
		return DateTime.fromMillis(ms).setZone(timezone);
	}

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

	const axisLabelCount = $derived(width < 560 ? 3 : 5);
	const axisLabels = $derived(
		buckets.length === 0
			? []
			: Array.from({ length: axisLabelCount }, (_, i) =>
					axisFormat(firstStart + (spanMs * i) / (axisLabelCount - 1))
				)
	);

	function rangeLabel(index: number): string {
		const start = zoned(Date.parse(buckets[index].start));
		const end = start.plus({ milliseconds: stepMs });
		if (stepMs >= 86400_000) {
			return stepMs === 86400_000
				? start.toLocaleString(DateTime.DATE_MED)
				: `${start.toLocaleString(DateTime.DATE_MED)} – ${end.toLocaleString(DateTime.DATE_MED)}`;
		}
		const time = stepMs < 60_000 ? DateTime.TIME_WITH_SECONDS : DateTime.TIME_SIMPLE;
		return `${start.toLocaleString(DateTime.DATE_MED)}, ${start.toLocaleString(time)} – ${end.toLocaleString(time)}`;
	}

	function indexAt(clientX: number, el: HTMLElement): number {
		const rect = el.getBoundingClientRect();
		const i = Math.floor(((clientX - rect.left) / rect.width) * buckets.length);
		return Math.max(0, Math.min(buckets.length - 1, i));
	}

	function handlePointerMove(e: PointerEvent) {
		if (buckets.length === 0) return;
		const i = indexAt(e.clientX, e.currentTarget as HTMLElement);
		hoverIndex = i;
		if (dragFrom !== null) dragTo = i;
	}

	function handlePointerDown(e: PointerEvent) {
		if (!onRangeSelect || buckets.length === 0 || e.button !== 0) return;
		const i = indexAt(e.clientX, e.currentTarget as HTMLElement);
		dragFrom = i;
		dragTo = i;
		(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
		e.preventDefault();
	}

	function handlePointerUp() {
		if (dragFrom === null || dragTo === null) return;
		const lo = Math.min(dragFrom, dragTo);
		const hi = Math.max(dragFrom, dragTo);
		dragFrom = null;
		dragTo = null;
		const from = new Date(Date.parse(buckets[lo].start));
		const to = new Date(Date.parse(buckets[hi].start) + stepMs - 1);
		onRangeSelect?.(from, to);
	}

	function cancelDrag() {
		dragFrom = null;
		dragTo = null;
	}

	$effect(() => {
		if (dragFrom === null) return;
		const onKey = (e: KeyboardEvent) => {
			if (e.key === 'Escape') cancelDrag();
		};
		window.addEventListener('keydown', onKey);
		return () => window.removeEventListener('keydown', onKey);
	});

	const selection = $derived(
		dragFrom !== null && dragTo !== null
			? { lo: Math.min(dragFrom, dragTo), hi: Math.max(dragFrom, dragTo) }
			: null
	);

	const hovered = $derived(hoverIndex !== null ? buckets[hoverIndex] : null);
	const tooltipLeft = $derived(
		hoverIndex !== null ? Math.max(0, Math.min(width, (hoverIndex + 0.5) * slot)) : 0
	);
</script>

<div class="flex flex-col gap-1.5 border-b px-3 pt-2 pb-2">
	<div class="flex min-h-4 flex-wrap items-center justify-between gap-x-4 gap-y-1 text-xs">
		<span class="text-muted-foreground tabular-nums">
			{#if buckets.length > 0}
				<span class="font-medium text-foreground">{total.toLocaleString()}</span>
				{total === 1 ? 'log' : 'logs'}
			{/if}
		</span>
		<div class="flex flex-wrap items-center gap-x-3 gap-y-1 text-muted-foreground">
			{#each levelTotals as level (level.key)}
				<span class="inline-flex items-center gap-1.5">
					<span class="h-2 w-2 rounded-full" style="background-color: {level.color};"></span>
					{level.label}
					<span class="text-foreground tabular-nums">{level.count.toLocaleString()}</span>
				</span>
			{/each}
		</div>
	</div>

	<div
		class="relative touch-none select-none {onRangeSelect && buckets.length > 0
			? 'cursor-pointer'
			: ''}"
		style:height="{height}px"
		bind:clientWidth={width}
		role="application"
		aria-label="Log volume by level over the selected time range. Click a bar or drag across bars to zoom in."
		onpointermove={handlePointerMove}
		onpointerleave={() => (hoverIndex = null)}
		onpointerdown={handlePointerDown}
		onpointerup={handlePointerUp}
		onpointercancel={cancelDrag}
	>
		{#if width > 0}
			<svg
				{width}
				{height}
				class="block overflow-visible transition-opacity {loading ? 'opacity-50' : ''}"
			>
				{#if selection}
					<rect
						x={selection.lo * slot}
						y={0}
						width={(selection.hi - selection.lo + 1) * slot}
						{height}
						fill="var(--primary)"
						fill-opacity="0.15"
						rx="3"
					/>
				{:else if hoverIndex !== null}
					<rect
						x={hoverIndex * slot}
						y={0}
						width={slot}
						{height}
						fill="currentColor"
						class="text-muted-foreground"
						fill-opacity="0.12"
						rx="2"
					/>
				{/if}
				{#each bars as bar, i (i)}
					{#if bar.segments.length === 0}
						<rect
							x={bar.x}
							y={height - EMPTY_HEIGHT}
							width={barWidth}
							height={EMPTY_HEIGHT}
							fill="var(--log-empty)"
							rx="1"
						/>
					{:else}
						{#each bar.segments as seg, j (j)}
							{#if seg.top}
								<path
									d={roundedTopPath(bar.x, seg.y, barWidth, seg.h)}
									fill={seg.color}
									fill-opacity={BAR_OPACITY}
								/>
							{:else}
								<rect
									x={bar.x}
									y={seg.y}
									width={barWidth}
									height={seg.h}
									fill={seg.color}
									fill-opacity={BAR_OPACITY}
								/>
							{/if}
						{/each}
					{/if}
				{/each}
			</svg>
		{/if}

		{#if hovered && hoverIndex !== null && !selection}
			{@const rows = LOG_VOLUME_LEVELS.filter((l) => hovered[l.key] > 0)}
			<div
				class="pointer-events-none absolute top-full z-20 mt-2 min-w-40 rounded-md border border-border bg-popover px-2.5 py-1.5 text-xs text-popover-foreground shadow-md"
				style="left: {tooltipLeft}px; transform: translateX({tooltipLeft < 90
					? '0'
					: tooltipLeft > width - 90
						? '-100%'
						: '-50%'});"
			>
				<div class="mb-1 whitespace-nowrap text-muted-foreground">{rangeLabel(hoverIndex)}</div>
				{#if rows.length === 0}
					<div class="text-muted-foreground">No logs</div>
				{:else}
					{#each rows as level (level.key)}
						<div class="flex items-center gap-1.5 whitespace-nowrap">
							<span class="h-2 w-2 rounded-full" style="background-color: {level.color};"></span>
							<span class="text-muted-foreground">{level.label}</span>
							<span class="ml-auto pl-3 font-medium tabular-nums">
								{hovered[level.key].toLocaleString()}
							</span>
						</div>
					{/each}
					{#if rows.length > 1}
						<div class="mt-1 flex justify-between border-t border-border pt-1">
							<span class="text-muted-foreground">Total</span>
							<span class="font-medium tabular-nums">{bucketTotal(hovered).toLocaleString()}</span>
						</div>
					{/if}
				{/if}
			</div>
		{/if}
	</div>

	<div class="flex justify-between text-[10px] leading-none text-muted-foreground tabular-nums">
		{#each axisLabels as label, i (i)}
			<span>{label}</span>
		{/each}
	</div>
</div>
