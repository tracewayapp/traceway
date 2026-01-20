<script lang="ts">
	import { scaleUtc, scaleLinear } from 'd3-scale';
	import { stack, area, stackOrderReverse } from 'd3-shape';
	import { min, max } from 'd3-array';
	import { formatDateTime } from '$lib/utils/formatters';
	import { getTimezone } from '$lib/state/timezone.svelte';

	type DataPoint = {
		timestamp: Date;
		endpoint: string;
		value: number;
	};

	type StackedData = {
		[key: string]: number | Date;
		timestamp: Date;
	};

	let {
		endpoints = [],
		series = [],
		height = 220,
		padding = { top: 10, right: 4, bottom: 20, left: 55 },
		unit = 'ms',
		formatValue,
		onRangeSelect
	} = $props<{
		endpoints: string[];
		series: DataPoint[];
		height?: number;
		padding?: { top: number; right: number; bottom: number; left: number };
		unit?: string;
		formatValue?: (value: number) => string;
		onRangeSelect?: (from: Date, to: Date) => void;
	}>();

	const tz = $derived(getTimezone());

	// Chart colors using CSS variables
	const chartColors = [
		'var(--chart-1)',
		'var(--chart-2)',
		'var(--chart-3)',
		'var(--chart-4)',
		'var(--chart-5)',
		'var(--muted-foreground)' // For "Other"
	];

	let containerRef = $state<HTMLDivElement | null>(null);
	let width = $state(300);

	// Hover state
	let isHovering = $state(false);
	let mouseX = $state(0);

	// Drag selection state
	let isDragging = $state(false);
	let dragStartX = $state(0);
	let dragEndX = $state(0);

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

	// Transform series data to stacked format
	const stackedData = $derived(() => {
		if (series.length === 0 || endpoints.length === 0) return [];

		// Group by timestamp
		const byTimestamp = new Map<number, StackedData>();

		for (const point of series) {
			const ts = point.timestamp.getTime();
			if (!byTimestamp.has(ts)) {
				const entry: StackedData = { timestamp: point.timestamp };
				// Initialize all endpoints to 0
				for (const ep of endpoints) {
					entry[ep] = 0;
				}
				byTimestamp.set(ts, entry);
			}
			const entry = byTimestamp.get(ts)!;
			entry[point.endpoint] = point.value;
		}

		// Sort by timestamp
		return Array.from(byTimestamp.values()).sort(
			(a, b) => a.timestamp.getTime() - b.timestamp.getTime()
		);
	});

	// Calculate domains
	const xMin = $derived(() => {
		const data = stackedData();
		if (data.length === 0) return new Date();
		return min(data, (d) => d.timestamp) ?? new Date();
	});

	const xMax = $derived(() => {
		const data = stackedData();
		if (data.length === 0) return new Date();
		return max(data, (d) => d.timestamp) ?? new Date();
	});

	// Helper to get numeric value from StackedData
	function getNumericValue(d: StackedData, key: string): number {
		const val = d[key];
		return typeof val === 'number' ? val : 0;
	}

	// Calculate max stacked value
	const yMax = $derived(() => {
		const data = stackedData();
		if (data.length === 0 || endpoints.length === 0) return 0;

		let maxSum = 0;
		for (const d of data) {
			let sum = 0;
			for (const ep of endpoints) {
				sum += getNumericValue(d, ep);
			}
			maxSum = Math.max(maxSum, sum);
		}
		return maxSum;
	});

	// Add padding to y domain
	const yDomainMax = $derived(yMax() * 1.1 || 1);

	// Scales
	const xScale = $derived(
		scaleUtc()
			.domain([xMin(), xMax()])
			.range([0, chartWidth])
	);

	const yScale = $derived(
		scaleLinear()
			.domain([0, yDomainMax])
			.range([chartHeight, 0])
	);

	// Generate Y axis ticks
	const yTicks = $derived(() => yScale.ticks(4));

	// Format Y axis label
	function formatYLabel(value: number): string {
		if (value >= 1000000) return (value / 1000000).toFixed(1) + 'M';
		if (value >= 1000) return (value / 1000).toFixed(1) + 'k';
		if (Number.isInteger(value)) return value.toString();
		return value.toFixed(1);
	}

	// Generate stacked area paths
	const stackedAreas = $derived(() => {
		const data = stackedData();
		if (data.length === 0 || endpoints.length === 0) return [];

		const stackGen = stack<StackedData>()
			.keys(endpoints)
			.order(stackOrderReverse);

		const stackedSeries = stackGen(data);

		const areaGen = area<[number, number]>()
			.x((d, i) => xScale(data[i].timestamp))
			.y0((d) => yScale(d[0]))
			.y1((d) => yScale(d[1]));

		return stackedSeries.map((s, i) => ({
			key: s.key,
			path: areaGen(s as unknown as [number, number][]) || '',
			color: chartColors[i % chartColors.length]
		}));
	});

	const hasData = $derived(stackedData().length > 0 && endpoints.length > 0);

	// Check if mouse X is within the chart area
	function isInChartArea(): boolean {
		return mouseX >= padding.left && mouseX <= width - padding.right;
	}

	// Calculate time based on X position
	function getTimeAtPosition(x: number): Date {
		if (chartWidth <= 0) return xMin();
		const chartX = x - padding.left;
		const percentage = Math.max(0, Math.min(1, chartX / chartWidth));
		const timeDiff = xMax().getTime() - xMin().getTime();
		return new Date(xMin().getTime() + timeDiff * percentage);
	}

	// Selection region computed values
	const selectionLeft = $derived(Math.max(padding.left, Math.min(dragStartX, dragEndX)));
	const selectionRight = $derived(Math.min(width - padding.right, Math.max(dragStartX, dragEndX)));
	const selectionWidth = $derived(selectionRight - selectionLeft);
	const selectionStartTime = $derived(getTimeAtPosition(selectionLeft));
	const selectionEndTime = $derived(getTimeAtPosition(selectionLeft + selectionWidth));

	// Find the closest data point to a given time
	function getDataAtTime(time: Date): StackedData | null {
		const data = stackedData();
		if (data.length === 0) return null;

		const targetMs = time.getTime();
		let closest: StackedData | null = null;
		let minDiff = Infinity;

		for (const d of data) {
			const diff = Math.abs(d.timestamp.getTime() - targetMs);
			if (diff < minDiff) {
				minDiff = diff;
				closest = d;
			}
		}

		return closest;
	}

	// Calculate time based on mouse X position
	const calculatedTime = $derived(() => {
		if (!isHovering) return null;
		return getTimeAtPosition(mouseX);
	});

	// Calculate the data point at the current hover position
	const calculatedData = $derived(() => {
		const time = calculatedTime();
		if (!time) return null;
		return getDataAtTime(time);
	});

	// Format value for display
	function formatDisplayValue(value: number): string {
		if (formatValue) return formatValue(value);
		if (Number.isInteger(value)) return value.toString();
		if (Math.abs(value) < 0.01) return value.toFixed(4);
		if (Math.abs(value) < 1) return value.toFixed(2);
		if (Math.abs(value) < 10) return value.toFixed(1);
		return Math.round(value).toString();
	}

	function handleMouseMove(e: MouseEvent) {
		if (!containerRef) return;
		const rect = containerRef.getBoundingClientRect();
		mouseX = e.clientX - rect.left;

		if (isDragging) {
			dragEndX = mouseX;
		}
	}

	function handleMouseEnter() {
		isHovering = true;
	}

	function handleMouseLeave() {
		isHovering = false;
		if (isDragging) isDragging = false;
	}

	function handleMouseDown(e: MouseEvent) {
		if (!containerRef || e.button !== 0) return;
		const rect = containerRef.getBoundingClientRect();
		const x = e.clientX - rect.left;

		if (x < padding.left || x > rect.width - padding.right) return;

		isDragging = true;
		dragStartX = x;
		dragEndX = x;
		e.preventDefault();
	}

	function handleMouseUp() {
		if (!isDragging) return;

		isDragging = false;

		if (selectionWidth > 10 && onRangeSelect) {
			const startTime = getTimeAtPosition(Math.min(dragStartX, dragEndX));
			const endTime = getTimeAtPosition(Math.max(dragStartX, dragEndX));
			onRangeSelect(startTime, endTime);
		}
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape' && isDragging) {
			isDragging = false;
			dragStartX = 0;
			dragEndX = 0;
		}
	}

	$effect(() => {
		if (isDragging) {
			window.addEventListener('keydown', handleKeydown);
			return () => window.removeEventListener('keydown', handleKeydown);
		}
	});

	function formatTime(date: Date | null): string {
		if (!date) return '';
		return formatDateTime(date, { timezone: tz, format: 'time' });
	}

	// Get color for an endpoint
	function getEndpointColor(endpoint: string): string {
		const idx = endpoints.indexOf(endpoint);
		if (idx === -1) return chartColors[chartColors.length - 1];
		return chartColors[idx % chartColors.length];
	}

	// Truncate long endpoint names for legend
	function truncateEndpoint(endpoint: string, maxLen: number = 30): string {
		if (endpoint.length <= maxLen) return endpoint;
		return endpoint.slice(0, maxLen - 3) + '...';
	}
</script>

<div class="flex flex-col gap-2">
	<div
		bind:this={containerRef}
		class="relative select-none w-full"
		style="height: {height}px; cursor: {isDragging ? 'col-resize' : (isInChartArea() ? 'crosshair' : 'default')};"
		onmouseenter={handleMouseEnter}
		onmouseleave={handleMouseLeave}
		onmousemove={handleMouseMove}
		onmousedown={handleMouseDown}
		onmouseup={handleMouseUp}
		role="application"
		aria-label="Stacked area chart with drag-to-zoom"
	>
		{#if hasData && chartWidth > 0}
			<svg {width} {height}>
				<g transform="translate({padding.left}, {padding.top})">
					<!-- Y axis grid lines -->
					{#each yTicks() as tick}
						<line
							x1={0}
							y1={yScale(tick)}
							x2={chartWidth}
							y2={yScale(tick)}
							stroke="currentColor"
							class="text-border"
							stroke-width="1"
							stroke-dasharray="4 4"
						/>
					{/each}

					<!-- Y axis labels -->
					{#each yTicks() as tick}
						<text
							x={-8}
							y={yScale(tick)}
							text-anchor="end"
							dominant-baseline="middle"
							fill="currentColor"
							class="text-muted-foreground"
							font-size="10"
						>
							{formatYLabel(tick)}
						</text>
					{/each}

					<!-- X axis line -->
					<line
						x1={0}
						y1={chartHeight}
						x2={chartWidth}
						y2={chartHeight}
						stroke="currentColor"
						class="text-border"
						stroke-width="1"
					/>

					<!-- Y axis line -->
					<line
						x1={0}
						y1={0}
						x2={0}
						y2={chartHeight}
						stroke="currentColor"
						class="text-border"
						stroke-width="1"
					/>

					<!-- Stacked areas -->
					{#each stackedAreas() as area}
						<path
							d={area.path}
							fill={area.color}
							fill-opacity="0.7"
							stroke={area.color}
							stroke-width="1"
						/>
					{/each}
				</g>
			</svg>
		{:else}
			<div class="flex h-full items-center justify-center text-sm text-muted-foreground">
				No data in this period
			</div>
		{/if}

		<!-- Drag selection overlay -->
		{#if isDragging && selectionWidth > 0}
			<div
				class="absolute top-0 bottom-0 bg-primary/20 border-x border-primary/40 pointer-events-none"
				style="left: {selectionLeft}px; width: {selectionWidth}px;"
			>
				<div class="absolute -top-5 left-0 -translate-x-full text-[9px] font-medium text-primary whitespace-nowrap">
					{formatTime(selectionStartTime)}
				</div>
				<div class="absolute -top-5 right-0 translate-x-full text-[9px] font-medium text-primary whitespace-nowrap">
					{formatTime(selectionEndTime)}
				</div>
			</div>
		{/if}

		<!-- Hover tooltip -->
		{#if isHovering && !isDragging && isInChartArea() && calculatedData()}
			{@const clampedX = Math.max(padding.left, Math.min(mouseX, width - padding.right))}
			{@const data = calculatedData()}

			<!-- Vertical line -->
			<div
				class="absolute top-0 bottom-0 w-px bg-muted-foreground/50 pointer-events-none"
				style="left: {clampedX}px;"
			></div>

			<!-- Value tooltip -->
			{#if data}
				<div
					class="absolute -translate-x-1/2 -translate-y-full pointer-events-none z-10"
					style="left: {clampedX}px; top: 10px;"
				>
					<div class="bg-background text-foreground border border-border rounded px-2 py-1.5 text-xs font-medium whitespace-nowrap mb-1 flex flex-col gap-0.5">
						{#each endpoints as endpoint}
							{@const value = getNumericValue(data, endpoint)}
							{#if value > 0}
								<div class="flex items-center gap-1.5">
									<span class="h-2 w-2 rounded-full flex-shrink-0" style="background-color: {getEndpointColor(endpoint)};"></span>
									<span class="truncate max-w-[200px]" title={endpoint}>{truncateEndpoint(endpoint, 25)}</span>
									<span class="ml-auto tabular-nums">{formatDisplayValue(value)}{!formatValue && unit ? ` ${unit}` : ''}</span>
								</div>
							{/if}
						{/each}
					</div>
				</div>
			{/if}

			<!-- Time label -->
			<div
				class="absolute bottom-0 -translate-x-1/2 translate-y-full pointer-events-none"
				style="left: {clampedX}px;"
			>
				<div class="bg-background border border-border rounded px-1.5 py-0.5 text-[10px] text-muted-foreground whitespace-nowrap shadow-sm mt-1">
					{formatTime(calculatedTime())}
				</div>
			</div>
		{/if}
	</div>

	<!-- Legend -->
	{#if hasData}
		<div class="flex flex-wrap gap-x-4 gap-y-1 px-2 text-xs text-muted-foreground">
			{#each endpoints as endpoint, i}
				<div class="flex items-center gap-1.5">
					<span class="h-2 w-2 rounded-full flex-shrink-0" style="background-color: {chartColors[i % chartColors.length]};"></span>
					<span class="truncate max-w-[180px]" title={endpoint}>{truncateEndpoint(endpoint, 25)}</span>
				</div>
			{/each}
		</div>
	{/if}
</div>
