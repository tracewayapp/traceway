<script module lang="ts">
	export type SeriesPoint = { timestamp: string; value: number };
	export type SeriesGroup = { key: string; points: SeriesPoint[] };

	const PALETTE = [
		'hsl(217, 91%, 60%)',
		'hsl(160, 60%, 45%)',
		'hsl(35, 92%, 55%)',
		'hsl(280, 65%, 60%)',
		'hsl(0, 72%, 58%)',
		'hsl(190, 80%, 45%)',
		'hsl(50, 90%, 50%)',
		'hsl(330, 70%, 60%)'
	];

	export function seriesColor(i: number): string {
		return PALETTE[i % PALETTE.length];
	}

	function clamp(x: number, lo: number, hi: number): number {
		return Math.max(lo, Math.min(hi, x));
	}
</script>

<script lang="ts">
	import { formatValue, formatRate } from '$lib/utils/profile-format';
	import { getTimezone } from '$lib/state/timezone.svelte';
	import { DateTime } from 'luxon';

	interface Props {
		series?: SeriesPoint[];
		groups?: SeriesGroup[];
		unit: string;
		isRate?: boolean;
		height?: number;
		onSelectRange?: (fromIso: string, toIso: string) => void;
	}

	let { series = [], groups = [], unit, isRate = false, height = 140, onSelectRange }: Props = $props();

	const timezone = $derived(getTimezone());

	let container = $state<HTMLDivElement>();
	let width = $state(0);

	$effect(() => {
		const el = container;
		if (!el || typeof ResizeObserver === 'undefined') return;
		const observer = new ResizeObserver((entries) => {
			width = entries[0].contentRect.width;
		});
		observer.observe(el);
		width = el.clientWidth;
		return () => observer.disconnect();
	});

	const padL = 6;
	const padR = 6;
	const padT = 10;
	const padB = 20;

	const lines = $derived.by(() => {
		if (groups.length) return groups.map((g, i) => ({ key: g.key, points: g.points, color: seriesColor(i) }));
		if (series.length) return [{ key: '', points: series, color: 'var(--primary)' }];
		return [] as { key: string; points: SeriesPoint[]; color: string }[];
	});

	const allPoints = $derived(lines.flatMap((l) => l.points));
	const times = $derived(allPoints.map((p) => new Date(p.timestamp).getTime()));
	const tMin = $derived(times.length ? Math.min(...times) : 0);
	const tMax = $derived(times.length ? Math.max(...times) : 0);
	const vMax = $derived(Math.max(1, ...allPoints.map((p) => p.value)));

	const plotW = $derived(Math.max(0, width - padL - padR));
	const plotH = $derived(Math.max(0, height - padT - padB));

	function xOf(ts: number): number {
		if (tMax === tMin) return padL;
		return padL + ((ts - tMin) / (tMax - tMin)) * plotW;
	}
	function yOf(v: number): number {
		return padT + plotH - (v / vMax) * plotH;
	}
	function pathFor(points: SeriesPoint[]): string {
		return points
			.map(
				(p, i) =>
					`${i === 0 ? 'M' : 'L'} ${xOf(new Date(p.timestamp).getTime()).toFixed(1)} ${yOf(p.value).toFixed(1)}`
			)
			.join(' ');
	}

	function fmt(v: number): string {
		return isRate ? formatRate(unit, v) : formatValue(unit, v);
	}
	function fmtTime(ts: number): string {
		const span = tMax - tMin;
		const dt = DateTime.fromMillis(ts).setZone(timezone);
		return span > 36 * 60 * 60 * 1000 ? dt.toFormat('MMM d HH:mm') : dt.toFormat('HH:mm');
	}

	let dragStart = $state<number | null>(null);
	let dragEnd = $state<number | null>(null);
	let hoverX = $state<number | null>(null);

	function localX(e: PointerEvent): number {
		const rect = container!.getBoundingClientRect();
		return e.clientX - rect.left;
	}

	function onPointerDown(e: PointerEvent) {
		if (!container || !onSelectRange) return;
		dragStart = localX(e);
		dragEnd = dragStart;
		(e.currentTarget as Element).setPointerCapture?.(e.pointerId);
	}
	function onPointerMove(e: PointerEvent) {
		if (!container) return;
		hoverX = localX(e);
		if (dragStart !== null) dragEnd = hoverX;
	}
	function onPointerUp() {
		const start = dragStart;
		const end = dragEnd;
		dragStart = null;
		dragEnd = null;
		if (start === null || end === null || !onSelectRange || tMax === tMin) return;
		const x0 = Math.min(start, end);
		const x1 = Math.max(start, end);
		if (x1 - x0 < 5) return;
		const toTs = (x: number) =>
			tMin + ((clamp(x, padL, padL + plotW) - padL) / plotW) * (tMax - tMin);
		onSelectRange(new Date(toTs(x0)).toISOString(), new Date(toTs(x1)).toISOString());
	}
	function onPointerLeave() {
		hoverX = null;
	}

	const hoverIdx = $derived.by(() => {
		if (hoverX === null || !lines.length || tMax === tMin) return -1;
		const ts = tMin + ((clamp(hoverX, padL, padL + plotW) - padL) / plotW) * (tMax - tMin);
		const base = lines[0].points;
		let best = 0;
		let bestD = Infinity;
		for (let i = 0; i < base.length; i++) {
			const d = Math.abs(new Date(base[i].timestamp).getTime() - ts);
			if (d < bestD) {
				bestD = d;
				best = i;
			}
		}
		return best;
	});
</script>

<div class="w-full">
	<div
		bind:this={container}
		class="relative w-full select-none"
		style="height: {height}px; touch-action: none;"
		role="presentation"
		onpointerdown={onPointerDown}
		onpointermove={onPointerMove}
		onpointerup={onPointerUp}
		onpointerleave={onPointerLeave}
	>
		{#if allPoints.length < 2}
			<div class="flex h-full items-center justify-center text-sm text-muted-foreground">
				Not enough data in this range.
			</div>
		{:else}
			<svg {width} {height} class="block {onSelectRange ? 'cursor-crosshair' : ''}">
				<line
					x1={padL}
					y1={padT}
					x2={padL}
					y2={padT + plotH}
					class="stroke-border"
					stroke-width="1"
				/>
				<line
					x1={padL}
					y1={padT + plotH}
					x2={padL + plotW}
					y2={padT + plotH}
					class="stroke-border"
					stroke-width="1"
				/>

				{#each lines as line (line.key)}
					<path
						d={pathFor(line.points)}
						fill="none"
						stroke={line.color}
						stroke-width="1.5"
						stroke-linejoin="round"
					/>
				{/each}

				{#if hoverIdx >= 0 && lines[0]?.points[hoverIdx]}
					{@const hx = xOf(new Date(lines[0].points[hoverIdx].timestamp).getTime())}
					<line x1={hx} y1={padT} x2={hx} y2={padT + plotH} class="stroke-muted-foreground/40" stroke-width="1" />
				{/if}

				{#if dragStart !== null && dragEnd !== null}
					{@const a = Math.min(dragStart, dragEnd)}
					{@const b = Math.max(dragStart, dragEnd)}
					<rect x={a} y={padT} width={Math.max(0, b - a)} height={plotH} class="fill-primary/15 stroke-primary/50" stroke-width="1" />
				{/if}

				<text x={padL} y={padT - 2} class="fill-muted-foreground" style="font-size: 10px;">{fmt(vMax)}</text>
				<text x={padL} y={height - 6} class="fill-muted-foreground" style="font-size: 10px;">{fmtTime(tMin)}</text>
				<text x={padL + plotW} y={height - 6} text-anchor="end" class="fill-muted-foreground" style="font-size: 10px;">{fmtTime(tMax)}</text>
			</svg>

			{#if hoverIdx >= 0 && lines[0]?.points[hoverIdx]}
				<div class="pointer-events-none absolute right-1 top-1 rounded border bg-popover px-2 py-1 text-xs text-popover-foreground shadow-sm">
					<div class="text-muted-foreground">{fmtTime(new Date(lines[0].points[hoverIdx].timestamp).getTime())}</div>
					{#each lines as line (line.key)}
						<div class="flex items-center gap-1.5 tabular-nums">
							<span class="inline-block h-2 w-2 rounded-sm" style="background: {line.color}"></span>
							{#if line.key}<span class="text-muted-foreground">{line.key}</span>{/if}
							<span>{fmt(line.points[hoverIdx]?.value ?? 0)}</span>
						</div>
					{/each}
				</div>
			{/if}
		{/if}
	</div>

	{#if groups.length}
		<div class="mt-1 flex flex-wrap gap-x-3 gap-y-1">
			{#each lines as line (line.key)}
				<span class="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
					<span class="inline-block h-2 w-2 rounded-sm" style="background: {line.color}"></span>
					<span class="max-w-[12rem] truncate">{line.key || '(empty)'}</span>
				</span>
			{/each}
		</div>
	{/if}

	{#if onSelectRange && allPoints.length >= 2}
		<div class="mt-1 text-center text-[11px] text-muted-foreground">Drag to zoom into a time range</div>
	{/if}
</div>
