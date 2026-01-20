<script lang="ts">
	import { scaleUtc, scaleLinear, type NumberValue } from 'd3-scale';
	import { line } from 'd3-shape';
	import { min, max } from 'd3-array';
	import { formatDateTime } from '$lib/utils/formatters';
	import { getTimezone } from '$lib/state/timezone.svelte';

	type DataPoint = {
		timestamp: Date;
		value: number;
	};

	type Series = {
		key: string;
		data: DataPoint[];
		color: string;
	};

	type ServerData = {
		serverName: string;
		trend: DataPoint[];
	};

	let {
		series = [],
		xDomain = null,
		height = 220,
		padding = { top: 10, right: 4, bottom: 20, left: 45 },
		// Overlay props
		onRangeSelect,
		data = [],
		servers = [],
		serverColorMap = {},
		unit = '',
		formatValue,
		sharedHoverTime = null,
		isSourceChart = false,
		onHoverTimeChange
	} = $props<{
		series: Series[];
		xDomain?: [Date, Date] | null;
		height?: number;
		padding?: { top: number; right: number; bottom: number; left: number };
		// Overlay props
		onRangeSelect?: (from: Date, to: Date) => void;
		data?: DataPoint[];
		servers?: ServerData[];
		serverColorMap?: Record<string, string>;
		unit?: string;
		formatValue?: (value: number) => string;
		sharedHoverTime?: Date | null;
		isSourceChart?: boolean;
		onHoverTimeChange?: (time: Date | null) => void;
	}>();

	const tz = $derived(getTimezone());

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

	// Calculate domains from data
	const allData = $derived(series.flatMap((s: Series) => s.data));

	const xMin = $derived(xDomain?.[0] ?? min(allData, (d: DataPoint) => d.timestamp) ?? new Date());
	const xMax = $derived(xDomain?.[1] ?? max(allData, (d: DataPoint) => d.timestamp) ?? new Date());

	const yMin = $derived(() => {
		const minVal = min(allData, (d: DataPoint) => d.value);
		return typeof minVal === 'number' ? minVal : 0;
	});

	const yMax = $derived(() => {
		const maxVal = max(allData, (d: DataPoint) => d.value);
		return typeof maxVal === 'number' ? maxVal : 0;
	});

	// Add padding to y domain
	const yPadding = $derived((yMax() - yMin()) * 0.1 || 1);
	const yDomainMin = $derived(Math.max(0, yMin() - yPadding));
	const yDomainMax = $derived(yMax() + yPadding);

	// Scales
	const xScale = $derived(
		scaleUtc()
			.domain([xMin, xMax])
			.range([0, chartWidth])
	);

	const yScale = $derived(
		scaleLinear()
			.domain([yDomainMin, yDomainMax])
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

	// Generate path string for a series
	function generatePath(seriesData: DataPoint[]): string {
		if (seriesData.length === 0) return '';

		const lineGen = line<DataPoint>()
			.x(d => xScale(d.timestamp))
			.y(d => yScale(d.value));

		return lineGen(seriesData) || '';
	}

	const hasData = $derived(allData.length > 0);

	// Check if we have multi-server data
	const hasMultiServerData = $derived(servers && servers.length > 1);

	// Check if mouse X is within the chart area
	function isInChartArea(): boolean {
		return mouseX >= padding.left && mouseX <= width - padding.right;
	}

	// Calculate time based on X position
	function getTimeAtPosition(x: number): Date {
		if (chartWidth <= 0) return xMin;
		const chartX = x - padding.left;
		const percentage = Math.max(0, Math.min(1, chartX / chartWidth));
		const timeDiff = xMax.getTime() - xMin.getTime();
		return new Date(xMin.getTime() + timeDiff * percentage);
	}

	// Calculate X position based on time
	function getPositionAtTime(time: Date): number {
		if (chartWidth <= 0) return padding.left;
		const timeDiff = xMax.getTime() - xMin.getTime();
		if (timeDiff <= 0) return padding.left;
		const percentage = (time.getTime() - xMin.getTime()) / timeDiff;
		return padding.left + (percentage * chartWidth);
	}

	// Calculate gap threshold from data (2x median interval)
	function getGapThreshold(pointData: DataPoint[]): number {
		if (pointData.length < 2) return 3600000;
		const intervals: number[] = [];
		for (let i = 1; i < Math.min(pointData.length, 10); i++) {
			intervals.push(pointData[i].timestamp.getTime() - pointData[i - 1].timestamp.getTime());
		}
		intervals.sort((a, b) => a - b);
		return intervals[Math.floor(intervals.length / 2)] * 2;
	}

	// Break data into line segments at gaps (for gap-aware rendering)
	function getLineSegments(pointData: DataPoint[]): DataPoint[][] {
		if (pointData.length === 0) return [];

		const threshold = getGapThreshold(pointData);
		const segments: DataPoint[][] = [];
		let currentSegment: DataPoint[] = [pointData[0]];

		for (let i = 1; i < pointData.length; i++) {
			const gap = pointData[i].timestamp.getTime() - pointData[i - 1].timestamp.getTime();
			if (gap > threshold) {
				if (currentSegment.length > 0) segments.push(currentSegment);
				currentSegment = [pointData[i]];
			} else {
				currentSegment.push(pointData[i]);
			}
		}

		if (currentSegment.length > 0) segments.push(currentSegment);
		return segments;
	}

	// Find isolated points (points with gaps on both sides)
	function getIsolatedPoints(pointData: DataPoint[]): DataPoint[] {
		if (pointData.length === 0) return [];
		if (pointData.length === 1) return pointData;

		const threshold = getGapThreshold(pointData);
		const isolated: DataPoint[] = [];

		for (let i = 0; i < pointData.length; i++) {
			const hasGapBefore = i === 0 ||
				(pointData[i].timestamp.getTime() - pointData[i - 1].timestamp.getTime() > threshold);
			const hasGapAfter = i === pointData.length - 1 ||
				(pointData[i + 1].timestamp.getTime() - pointData[i].timestamp.getTime() > threshold);

			if (hasGapBefore && hasGapAfter) {
				isolated.push(pointData[i]);
			}
		}

		return isolated;
	}

	// Find the value at a given time, interpolating for line sections
	function getValueAtTime(time: Date, pointData: DataPoint[]): { value: number; isInterpolated: boolean } | null {
		if (pointData.length === 0) return null;

		const targetMs = time.getTime();
		const threshold = getGapThreshold(pointData);

		let leftIdx = -1;
		let rightIdx = -1;

		for (let i = 0; i < pointData.length; i++) {
			const pointMs = pointData[i].timestamp.getTime();
			if (pointMs <= targetMs) leftIdx = i;
			if (pointMs >= targetMs && rightIdx === -1) rightIdx = i;
		}

		if (leftIdx >= 0 && pointData[leftIdx].timestamp.getTime() === targetMs) {
			return { value: pointData[leftIdx].value, isInterpolated: false };
		}

		if (leftIdx >= 0 && rightIdx >= 0 && leftIdx !== rightIdx) {
			const leftPoint = pointData[leftIdx];
			const rightPoint = pointData[rightIdx];
			const gap = rightPoint.timestamp.getTime() - leftPoint.timestamp.getTime();

			if (gap <= threshold) {
				const t = (targetMs - leftPoint.timestamp.getTime()) / gap;
				const interpolatedValue = leftPoint.value + t * (rightPoint.value - leftPoint.value);
				return { value: interpolatedValue, isInterpolated: true };
			}
		}

		if (leftIdx >= 0) {
			const distToLeft = targetMs - pointData[leftIdx].timestamp.getTime();
			if (distToLeft <= threshold / 2) {
				return { value: pointData[leftIdx].value, isInterpolated: false };
			}
		}

		if (rightIdx >= 0) {
			const distToRight = pointData[rightIdx].timestamp.getTime() - targetMs;
			if (distToRight <= threshold / 2) {
				return { value: pointData[rightIdx].value, isInterpolated: false };
			}
		}

		return null;
	}

	// Calculate time based on mouse X position
	const calculatedTime = $derived(() => {
		if (!isHovering) return null;
		return getTimeAtPosition(mouseX);
	});

	// Calculate the value at the current hover position
	const calculatedValue = $derived(() => {
		const time = calculatedTime();
		if (!time) return null;
		return getValueAtTime(time, data);
	});

	// Calculate values for all servers at the current hover position
	const calculatedServerValues = $derived(() => {
		const time = calculatedTime();
		if (!time || !hasMultiServerData) return [];

		const results: { serverName: string; value: number; color: string }[] = [];

		for (const server of servers) {
			const valueData = getValueAtTime(time, server.trend);
			if (valueData) {
				results.push({
					serverName: server.serverName,
					value: valueData.value,
					color: serverColorMap[server.serverName] || '#888888'
				});
			}
		}

		return results;
	});

	// Shadow line
	const shouldShowShadowLine = $derived(
		sharedHoverTime !== null && !isSourceChart && !isDragging && !isHovering
	);

	const shadowLineX = $derived(() => {
		if (!shouldShowShadowLine || !sharedHoverTime) return 0;
		return getPositionAtTime(sharedHoverTime);
	});

	// Format value for display - returns { text, includesUnit }
	function formatDisplayValue(value: number): { text: string; includesUnit: boolean } {
		if (formatValue) return { text: formatValue(value), includesUnit: true };
		if (Number.isInteger(value)) return { text: value.toString(), includesUnit: false };
		if (Math.abs(value) < 0.01) return { text: value.toFixed(4), includesUnit: false };
		if (Math.abs(value) < 1) return { text: value.toFixed(2), includesUnit: false };
		if (Math.abs(value) < 10) return { text: value.toFixed(1), includesUnit: false };
		return { text: Math.round(value).toString(), includesUnit: false };
	}

	// Selection region computed values
	const selectionLeft = $derived(Math.max(padding.left, Math.min(dragStartX, dragEndX)));
	const selectionRight = $derived(() => Math.min(width - padding.right, Math.max(dragStartX, dragEndX)));
	const selectionWidth = $derived(selectionRight() - selectionLeft);
	const selectionStartTime = $derived(() => getTimeAtPosition(selectionLeft));
	const selectionEndTime = $derived(() => getTimeAtPosition(selectionLeft + selectionWidth));

	function handleMouseMove(e: MouseEvent) {
		if (!containerRef) return;
		const rect = containerRef.getBoundingClientRect();
		mouseX = e.clientX - rect.left;

		if (isDragging) {
			dragEndX = mouseX;
		} else if (isInChartArea()) {
			onHoverTimeChange?.(getTimeAtPosition(mouseX));
		}
	}

	function handleMouseEnter() {
		isHovering = true;
	}

	function handleMouseLeave() {
		isHovering = false;
		if (isDragging) isDragging = false;
		onHoverTimeChange?.(null);
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

	function formatTime(date: Date | null): string {
		if (!date) return '';
		return formatDateTime(date, { timezone: tz, format: 'time' });
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
</script>

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
	aria-label="Chart with drag-to-zoom"
>
	{#if hasData && chartWidth > 0}
		{@const clipId = `chart-clip-${Math.random().toString(36).slice(2, 9)}`}
		<svg {width} {height}>
			<defs>
				<clipPath id={clipId}>
					<rect x={0} y={0} width={chartWidth} height={chartHeight} />
				</clipPath>
			</defs>
			<g transform="translate({padding.left}, {padding.top})">
				<!-- Y axis grid lines -->
				{#each yTicks() as tick}
					<line
						x1={0}
						y1={yScale(tick)}
						x2={chartWidth}
						y2={yScale(tick)}
						stroke="#e5e7eb"
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
						fill="#6b7280"
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
					stroke="#e5e7eb"
					stroke-width="1"
				/>

				<!-- Y axis line -->
				<line
					x1={0}
					y1={0}
					x2={0}
					y2={chartHeight}
					stroke="#e5e7eb"
					stroke-width="1"
				/>

				<!-- Lines for each series (with gap detection) -->
				<g clip-path="url(#{clipId})">
					{#each series as s}
						{#if s.data.length > 0}
							{@const segments = getLineSegments(s.data)}
							{@const isolated = getIsolatedPoints(s.data)}

							<!-- Draw line segments (only segments with 2+ points) -->
							{#each segments as segment}
								{#if segment.length > 1}
									<path
										d={generatePath(segment)}
										fill="none"
										stroke={s.color}
										stroke-width="1.5"
									/>
								{/if}
							{/each}

							<!-- Draw isolated points as dots -->
							{#each isolated as point}
								<circle
									cx={xScale(point.timestamp)}
									cy={yScale(point.value)}
									r="3"
									fill={s.color}
								/>
							{/each}
						{/if}
					{/each}
				</g>
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
				{formatTime(selectionStartTime())}
			</div>
			<div class="absolute -top-5 right-0 translate-x-full text-[9px] font-medium text-primary whitespace-nowrap">
				{formatTime(selectionEndTime())}
			</div>
		</div>
	{/if}

	<!-- Hover tooltip -->
	{#if isHovering && !isDragging && isInChartArea()}
		{@const clampedX = Math.max(padding.left, Math.min(mouseX, width - padding.right))}
		{@const valueData = calculatedValue()}

		<!-- Vertical line -->
		<div
			class="absolute top-0 bottom-0 w-px bg-muted-foreground/50 pointer-events-none"
			style="left: {clampedX}px;"
		></div>

		<!-- Value tooltip -->
		{#if hasMultiServerData && calculatedServerValues().length > 0}
			<div
				class="absolute top-0 -translate-x-1/2 -translate-y-full pointer-events-none"
				style="left: {clampedX}px;"
			>
				<div class="bg-foreground text-background rounded px-2 py-1 text-xs font-medium whitespace-nowrap shadow-lg mb-1 flex flex-col gap-0.5">
					{#each calculatedServerValues() as serverValue}
						{@const formatted = formatDisplayValue(serverValue.value)}
						<div class="flex items-center gap-1.5">
							<span class="h-2 w-2 rounded-full flex-shrink-0" style="background-color: {serverValue.color};"></span>
							<span>{formatted.text}{#if unit && !formatted.includesUnit}<span class="text-background/70 ml-0.5">{unit}</span>{/if}</span>
						</div>
					{/each}
				</div>
			</div>
		{:else if valueData}
			{@const formatted = formatDisplayValue(valueData.value)}
			<div
				class="absolute top-0 -translate-x-1/2 -translate-y-full pointer-events-none"
				style="left: {clampedX}px;"
			>
				<div class="bg-foreground text-background rounded px-2 py-1 text-xs font-medium whitespace-nowrap shadow-lg mb-1">
					{formatted.text}{#if unit && !formatted.includesUnit}<span class="text-background/70 ml-0.5">{unit}</span>{/if}
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

	<!-- Shadow line from other charts -->
	{#if shouldShowShadowLine}
		{@const shadowX = shadowLineX()}
		{#if shadowX >= padding.left && shadowX <= width - padding.right}
			<div
				class="absolute top-0 bottom-0 w-px bg-muted-foreground/25 pointer-events-none"
				style="left: {shadowX}px;"
			></div>
		{/if}
	{/if}
</div>
