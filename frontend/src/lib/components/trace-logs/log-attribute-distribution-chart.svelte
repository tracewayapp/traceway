<script lang="ts" module>
	export type LogAttributeBin = { start: number; end: number; count: number };
</script>

<script lang="ts">
	import { scaleLinear } from 'd3-scale';
	import { formatChartNumber } from '$lib/utils/log-attribute-chart';

	let {
		bins,
		binWidth,
		integral,
		markers,
		loading = false,
		height = 180
	}: {
		bins: LogAttributeBin[];
		binWidth: number;
		integral: boolean;
		markers: { label: string; value: number; color: string }[];
		loading?: boolean;
		height?: number;
	} = $props();

	const PAD = { top: 18, right: 8, bottom: 4, left: 52 };
	const RADIUS = 2;

	let width = $state(0);
	let hoverIndex = $state<number | null>(null);

	const plotW = $derived(Math.max(0, width - PAD.left - PAD.right));
	const plotH = $derived(Math.max(0, height - PAD.top - PAD.bottom));
	const total = $derived(bins.reduce((sum, b) => sum + b.count, 0));
	const lo = $derived(bins.length > 0 ? bins[0].start : 0);
	const hi = $derived(bins.length > 0 ? bins[bins.length - 1].end : 1);

	const x = $derived(scaleLinear().domain([lo, hi]).range([0, plotW]));
	const y = $derived(
		scaleLinear()
			.domain([0, Math.max(1, ...bins.map((b) => b.count))])
			.nice(4)
			.range([plotH, 0])
	);
	const yTicks = $derived(y.ticks(4).filter((t) => Number.isInteger(t)));

	// One whole number per bin reads as that number, not as a range.
	const singleValueBins = $derived(integral && binWidth === 1);

	const xLabels = $derived.by(() => {
		if (bins.length === 0) return [];
		const want = width < 560 ? 4 : 7;
		const every = Math.max(1, Math.ceil(bins.length / want));
		const labels: { x: number; text: string }[] = [];
		for (let i = 0; i < bins.length; i += every) {
			const b = bins[i];
			labels.push(
				singleValueBins
					? { x: x(b.start + binWidth / 2), text: formatChartNumber(b.start) }
					: { x: x(b.start), text: formatChartNumber(b.start) }
			);
		}
		if (!singleValueBins) labels.push({ x: x(hi), text: formatChartNumber(hi) });
		return labels;
	});

	function barPath(bx: number, by: number, w: number, h: number): string {
		const r = Math.min(RADIUS, w / 2, h);
		return `M${bx},${by + h}V${by + r}Q${bx},${by} ${bx + r},${by}H${bx + w - r}Q${bx + w},${by} ${bx + w},${by + r}V${by + h}Z`;
	}

	function handlePointerMove(e: PointerEvent) {
		if (bins.length === 0) return;
		const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
		const px = e.clientX - rect.left - PAD.left;
		const i = Math.floor((px / plotW) * bins.length);
		hoverIndex = i >= 0 && i < bins.length ? i : null;
	}

	const hovered = $derived(hoverIndex !== null ? bins[hoverIndex] : null);
	const tooltipLeft = $derived(hovered ? PAD.left + x((hovered.start + hovered.end) / 2) : 0);

	// Percentiles close together get their labels on two rows.
	const placedMarkers = $derived.by(() => {
		const sorted = [...markers].sort((a, b) => a.value - b.value);
		const out: ((typeof markers)[number] & { mx: number; row: number })[] = [];
		for (const m of sorted) {
			const mx = x(m.value);
			const prev = out[out.length - 1];
			const row = prev && mx - prev.mx < 30 && prev.row === 0 ? 1 : 0;
			out.push({ ...m, mx, row });
		}
		return out.filter((m) => m.mx >= 0 && m.mx <= plotW);
	});

	function binLabel(b: LogAttributeBin): string {
		if (singleValueBins) return formatChartNumber(b.start);
		return `${formatChartNumber(b.start)} to ${formatChartNumber(b.end)}`;
	}
</script>

<div class="flex flex-col gap-1">
	<div
		class="relative select-none"
		style:height="{height}px"
		bind:clientWidth={width}
		role="img"
		aria-label="How the values of the attribute are distributed"
		onpointermove={handlePointerMove}
		onpointerleave={() => (hoverIndex = null)}
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
							{formatChartNumber(tick)}
						</text>
					{/each}
					{#each bins as b, i (i)}
						{@const bx = x(b.start)}
						{@const bw = Math.max(1, x(b.end) - bx - 1)}
						{#if b.count > 0}
							{@const bh = Math.max(2, plotH - y(b.count))}
							<path
								d={barPath(bx + 0.5, plotH - bh, bw, bh)}
								fill="var(--chart-1)"
								fill-opacity={hoverIndex === i ? 1 : 0.8}
							/>
						{/if}
					{/each}
					{#each placedMarkers as m (m.label)}
						<line
							x1={m.mx}
							x2={m.mx}
							y1={-4}
							y2={plotH}
							stroke={m.color}
							stroke-width="1.5"
							stroke-dasharray="4 3"
						/>
						<text
							x={m.row === 0 ? m.mx : m.mx + 4}
							y={m.row === 0 ? -8 : 8}
							text-anchor={m.row === 0 ? 'middle' : 'start'}
							font-size="10"
							font-weight="500"
							fill={m.color}
						>
							{m.label}
						</text>
					{/each}
				</g>
			</svg>
		{/if}

		{#if hovered}
			<div
				class="pointer-events-none absolute top-6 z-20 min-w-40 rounded-md border border-border bg-popover px-2.5 py-1.5 text-xs text-popover-foreground shadow-md"
				style="left: {tooltipLeft}px; transform: translateX({tooltipLeft > width - 180
					? 'calc(-100% - 12px)'
					: '12px'});"
			>
				<div class="mb-1 whitespace-nowrap text-muted-foreground">{binLabel(hovered)}</div>
				<div class="flex justify-between gap-3 whitespace-nowrap">
					<span class="text-muted-foreground">Values</span>
					<span class="font-medium tabular-nums">{hovered.count.toLocaleString()}</span>
				</div>
				<div class="flex justify-between gap-3 whitespace-nowrap">
					<span class="text-muted-foreground">Share</span>
					<span class="font-medium tabular-nums">
						{total > 0 ? ((hovered.count / total) * 100).toFixed(1) : '0'}%
					</span>
				</div>
			</div>
		{/if}
	</div>
	<div class="relative h-3 text-[10px] leading-none text-muted-foreground tabular-nums">
		{#each xLabels as label, i (i)}
			<span class="absolute -translate-x-1/2 whitespace-nowrap" style:left="{PAD.left + label.x}px">
				{label.text}
			</span>
		{/each}
	</div>
</div>
