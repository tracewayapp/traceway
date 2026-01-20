<script lang="ts">
	import { scaleLinear, scaleBand } from 'd3-scale';
	import { max } from 'd3-array';

	type BarData = {
		endpoint: string;
		value: number;
	};

	let {
		data = [],
		height = 220,
		padding = { top: 10, right: 70, bottom: 10, left: 160 },
		unit = 'ms',
		formatValue
	} = $props<{
		data: BarData[];
		height?: number;
		padding?: { top: number; right: number; bottom: number; left: number };
		unit?: string;
		formatValue?: (value: number) => string;
	}>();

	// Chart colors using CSS variables
	const chartColors = [
		'var(--chart-1)',
		'var(--chart-2)',
		'var(--chart-3)',
		'var(--chart-4)',
		'var(--chart-5)',
		'hsl(280, 65%, 60%)',
		'hsl(200, 65%, 55%)',
		'hsl(340, 65%, 55%)'
	];

	let containerRef = $state<HTMLDivElement | null>(null);
	let width = $state(400);

	// Hover state for tooltip
	let hoveredIndex = $state<number | null>(null);

	// Observe container width
	$effect(() => {
		if (!containerRef) return;

		const observer = new ResizeObserver((entries) => {
			for (const entry of entries) {
				width = entry.contentRect.width;
			}
		});

		observer.observe(containerRef);
		return () => observer.disconnect();
	});

	// Chart dimensions
	const chartWidth = $derived(Math.max(0, width - padding.left - padding.right));
	const chartHeight = $derived(Math.max(0, height - padding.top - padding.bottom));

	// Sort data by value descending (worst at top)
	const sortedData = $derived(
		[...data].sort((a, b) => b.value - a.value)
	);

	// Calculate max value for scale
	const maxValue = $derived(max(sortedData, d => d.value) ?? 0);

	// X scale (value)
	const xScale = $derived(
		scaleLinear()
			.domain([0, maxValue * 1.1 || 1])
			.range([0, chartWidth])
	);

	// Y scale (endpoints)
	const yScale = $derived(
		scaleBand<string>()
			.domain(sortedData.map(d => d.endpoint))
			.range([0, chartHeight])
			.padding(0.25)
	);

	const barHeight = $derived(yScale.bandwidth());
	const hasData = $derived(sortedData.length > 0);

	// Format value for display
	function formatDisplayValue(value: number): string {
		if (formatValue) return formatValue(value);
		if (value >= 1000) {
			return `${(value / 1000).toFixed(1)}s`;
		}
		return `${Math.round(value)}${unit}`;
	}

	// Truncate endpoint name for display
	function truncateEndpoint(endpoint: string, maxLen: number = 22): string {
		if (endpoint.length <= maxLen) return endpoint;
		return endpoint.slice(0, maxLen - 3) + '...';
	}

	function handleLabelMouseEnter(index: number) {
		hoveredIndex = index;
	}

	function handleLabelMouseLeave() {
		hoveredIndex = null;
	}
</script>

<div
	bind:this={containerRef}
	class="relative w-full"
	style="height: {height}px;"
>
	{#if hasData && chartWidth > 0}
		<svg {width} {height}>
			<g transform="translate({padding.left}, {padding.top})">
				<!-- Bars -->
				{#each sortedData as item, i}
					{@const y = yScale(item.endpoint) ?? 0}
					{@const barWidth = xScale(item.value)}
					{@const color = chartColors[i % chartColors.length]}
					{@const isTruncated = item.endpoint.length > 22}

					<!-- Bar -->
					<rect
						x={0}
						{y}
						width={Math.max(0, barWidth)}
						height={barHeight}
						fill={color}
						fill-opacity="0.85"
						rx="3"
						ry="3"
					/>

					<!-- Endpoint label (left side) -->
					<text
						x={-8}
						y={y + barHeight / 2}
						text-anchor="end"
						dominant-baseline="middle"
						fill="currentColor"
						class="text-muted-foreground"
						font-size="11"
						style="cursor: default;"
						onmouseenter={() => isTruncated && handleLabelMouseEnter(i)}
						onmouseleave={handleLabelMouseLeave}
					>
						{truncateEndpoint(item.endpoint)}
					</text>

					<!-- Value label (right of bar) -->
					<text
						x={barWidth + 8}
						y={y + barHeight / 2}
						text-anchor="start"
						dominant-baseline="middle"
						fill="currentColor"
						class="text-foreground"
						font-size="11"
						font-weight="500"
						style="cursor: default;"
					>
						{formatDisplayValue(item.value)}
					</text>
				{/each}
			</g>
		</svg>

		<!-- Expanded label tooltip -->
		{#if hoveredIndex !== null}
			{@const item = sortedData[hoveredIndex]}
			{@const y = (yScale(item.endpoint) ?? 0) + padding.top + barHeight / 2}
			<div
				class="absolute z-10 pointer-events-none bg-background border border-border rounded px-2 py-1 text-xs text-muted-foreground whitespace-nowrap shadow-sm"
				style="right: {width - padding.left + 8}px; top: {y}px; transform: translateY(-50%);"
			>
				{item.endpoint}
			</div>
		{/if}
	{:else}
		<div class="flex h-full items-center justify-center text-sm text-muted-foreground">
			No data available
		</div>
	{/if}
</div>
