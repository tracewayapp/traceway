<script lang="ts">
	import type { Snippet } from 'svelte';
	import { formatDateTime } from '$lib/utils/formatters';
	import { getTimezone } from '$lib/state/timezone.svelte';

	type DataPoint = {
		timestamp: Date;
		value: number;
	};

	type ServerData = {
		serverName: string;
		trend: DataPoint[];
	};

	let {
		fromTime,
		toTime,
		children,
		onRangeSelect,
		chartPadding = { left: 20, right: 4 },
		data = [],
		servers = [],
		serverColorMap = {},
		unit = '',
		formatValue,
		timezone,
		chartId = '',
		sharedHoverTime = null,
		isSourceChart = false,
		onHoverTimeChange
	} = $props<{
		fromTime: Date;
		toTime: Date;
		children: Snippet;
		onRangeSelect?: (from: Date, to: Date) => void;
		chartPadding?: { left: number; right: number };
		data?: DataPoint[];
		servers?: ServerData[];
		serverColorMap?: Record<string, string>;
		unit?: string;
		formatValue?: (value: number) => string;
		timezone?: string;
		chartId?: string;
		sharedHoverTime?: Date | null;
		isSourceChart?: boolean;
		onHoverTimeChange?: (time: Date | null) => void;
	}>();

	// Check if we have multi-server data
	const hasMultiServerData = $derived(servers && servers.length > 1);

	const tz = $derived(timezone ?? getTimezone());

	let containerRef = $state<HTMLDivElement | null>(null);
	let isHovering = $state(false);
	let mouseX = $state(0);
	let mouseY = $state(0);

	// Drag selection state
	let isDragging = $state(false);
	let dragStartX = $state(0);
	let dragEndX = $state(0);

	// Calculate gap threshold from data (2x median interval)
	const gapThresholdMs = $derived(() => {
		if (data.length < 2) return 3600000; // 1 hour default
		const intervals: number[] = [];
		for (let i = 1; i < Math.min(data.length, 10); i++) {
			intervals.push(data[i].timestamp.getTime() - data[i - 1].timestamp.getTime());
		}
		intervals.sort((a, b) => a - b);
		const median = intervals[Math.floor(intervals.length / 2)];
		return median * 2;
	});

	// Get container width
	function getContainerWidth(): number {
		return containerRef?.getBoundingClientRect().width ?? 0;
	}

	// Get the actual chart plotting area width (container minus padding on both sides)
	function getChartAreaWidth(): number {
		return getContainerWidth() - chartPadding.left - chartPadding.right;
	}

	// Check if mouse X is within the chart area
	function isInChartArea(): boolean {
		return mouseX >= chartPadding.left && mouseX <= getContainerWidth() - chartPadding.right;
	}

	// Calculate time based on X position (relative to chart area, not container)
	function getTimeAtPosition(x: number): Date {
		if (!containerRef) return fromTime;
		const chartAreaWidth = getChartAreaWidth();
		if (chartAreaWidth <= 0) return fromTime;
		// Adjust x to be relative to chart area start
		const chartX = x - chartPadding.left;
		const percentage = Math.max(0, Math.min(1, chartX / chartAreaWidth));
		const timeDiff = toTime.getTime() - fromTime.getTime();
		return new Date(fromTime.getTime() + timeDiff * percentage);
	}

	// Calculate X position based on time (inverse of getTimeAtPosition)
	function getPositionAtTime(time: Date): number {
		if (!containerRef) return chartPadding.left;
		const chartAreaWidth = getChartAreaWidth();
		if (chartAreaWidth <= 0) return chartPadding.left;
		const timeDiff = toTime.getTime() - fromTime.getTime();
		if (timeDiff <= 0) return chartPadding.left;
		const percentage = (time.getTime() - fromTime.getTime()) / timeDiff;
		return chartPadding.left + (percentage * chartAreaWidth);
	}

	// Find the value at a given time, interpolating for line sections
	function getValueAtTime(time: Date): { value: number; isInterpolated: boolean } | null {
		if (data.length === 0) return null;

		const targetMs = time.getTime();
		const threshold = gapThresholdMs();

		// Find the bracketing points
		let leftIdx = -1;
		let rightIdx = -1;

		for (let i = 0; i < data.length; i++) {
			const pointMs = data[i].timestamp.getTime();
			if (pointMs <= targetMs) {
				leftIdx = i;
			}
			if (pointMs >= targetMs && rightIdx === -1) {
				rightIdx = i;
			}
		}

		// If exact match
		if (leftIdx >= 0 && data[leftIdx].timestamp.getTime() === targetMs) {
			return { value: data[leftIdx].value, isInterpolated: false };
		}

		// If we have both bracketing points
		if (leftIdx >= 0 && rightIdx >= 0 && leftIdx !== rightIdx) {
			const leftPoint = data[leftIdx];
			const rightPoint = data[rightIdx];
			const gap = rightPoint.timestamp.getTime() - leftPoint.timestamp.getTime();

			// Check if this is a continuous line section (no gap)
			if (gap <= threshold) {
				// Interpolate linearly
				const t = (targetMs - leftPoint.timestamp.getTime()) / gap;
				const interpolatedValue = leftPoint.value + t * (rightPoint.value - leftPoint.value);
				return { value: interpolatedValue, isInterpolated: true };
			}
		}

		// Check if we're close to a single point (for isolated points or edges)
		if (leftIdx >= 0) {
			const leftPoint = data[leftIdx];
			const distToLeft = targetMs - leftPoint.timestamp.getTime();
			if (distToLeft <= threshold / 2) {
				return { value: leftPoint.value, isInterpolated: false };
			}
		}

		if (rightIdx >= 0) {
			const rightPoint = data[rightIdx];
			const distToRight = rightPoint.timestamp.getTime() - targetMs;
			if (distToRight <= threshold / 2) {
				return { value: rightPoint.value, isInterpolated: false };
			}
		}

		return null; // In a gap, no value to show
	}

	// Calculate time based on mouse X position (for hover display)
	const calculatedTime = $derived(() => {
		if (!containerRef || !isHovering) return null;
		return getTimeAtPosition(mouseX);
	});

	// Calculate the value at the current hover position (for single-server)
	const calculatedValue = $derived(() => {
		const time = calculatedTime();
		if (!time) return null;
		return getValueAtTime(time);
	});

	// Get value at time for a specific server's data
	function getServerValueAtTime(serverData: DataPoint[], time: Date): { value: number; isInterpolated: boolean } | null {
		if (serverData.length === 0) return null;

		const targetMs = time.getTime();

		// Calculate threshold for this server
		let threshold = 3600000;
		if (serverData.length >= 2) {
			const intervals: number[] = [];
			for (let i = 1; i < Math.min(serverData.length, 10); i++) {
				intervals.push(serverData[i].timestamp.getTime() - serverData[i - 1].timestamp.getTime());
			}
			intervals.sort((a, b) => a - b);
			threshold = intervals[Math.floor(intervals.length / 2)] * 2;
		}

		// Find the bracketing points
		let leftIdx = -1;
		let rightIdx = -1;

		for (let i = 0; i < serverData.length; i++) {
			const pointMs = serverData[i].timestamp.getTime();
			if (pointMs <= targetMs) {
				leftIdx = i;
			}
			if (pointMs >= targetMs && rightIdx === -1) {
				rightIdx = i;
			}
		}

		// If exact match
		if (leftIdx >= 0 && serverData[leftIdx].timestamp.getTime() === targetMs) {
			return { value: serverData[leftIdx].value, isInterpolated: false };
		}

		// If we have both bracketing points
		if (leftIdx >= 0 && rightIdx >= 0 && leftIdx !== rightIdx) {
			const leftPoint = serverData[leftIdx];
			const rightPoint = serverData[rightIdx];
			const gap = rightPoint.timestamp.getTime() - leftPoint.timestamp.getTime();

			if (gap <= threshold) {
				const t = (targetMs - leftPoint.timestamp.getTime()) / gap;
				const interpolatedValue = leftPoint.value + t * (rightPoint.value - leftPoint.value);
				return { value: interpolatedValue, isInterpolated: true };
			}
		}

		// Check if we're close to a single point
		if (leftIdx >= 0) {
			const leftPoint = serverData[leftIdx];
			const distToLeft = targetMs - leftPoint.timestamp.getTime();
			if (distToLeft <= threshold / 2) {
				return { value: leftPoint.value, isInterpolated: false };
			}
		}

		if (rightIdx >= 0) {
			const rightPoint = serverData[rightIdx];
			const distToRight = rightPoint.timestamp.getTime() - targetMs;
			if (distToRight <= threshold / 2) {
				return { value: rightPoint.value, isInterpolated: false };
			}
		}

		return null;
	}

	// Calculate values for all servers at the current hover position
	const calculatedServerValues = $derived(() => {
		const time = calculatedTime();
		if (!time || !hasMultiServerData) return [];

		const results: { serverName: string; value: number; color: string }[] = [];

		for (const server of servers) {
			const valueData = getServerValueAtTime(server.trend, time);
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

	// Shadow line: show when another chart is being hovered
	const shouldShowShadowLine = $derived(
		sharedHoverTime !== null && !isSourceChart && !isDragging && !isHovering
	);

	const shadowLineX = $derived(() => {
		if (!shouldShowShadowLine || !sharedHoverTime) return 0;
		return getPositionAtTime(sharedHoverTime);
	});

	// Format value for display
	function formatDisplayValue(value: number): string {
		if (formatValue) {
			return formatValue(value);
		}
		// Default formatting based on value magnitude
		if (Number.isInteger(value)) return value.toString();
		if (Math.abs(value) < 0.01) return value.toFixed(4);
		if (Math.abs(value) < 1) return value.toFixed(2);
		if (Math.abs(value) < 10) return value.toFixed(1);
		return Math.round(value).toString();
	}

	// Selection region computed values (clamped to chart area)
	const selectionLeft = $derived(Math.max(chartPadding.left, Math.min(dragStartX, dragEndX)));
	const selectionRight = $derived(() => {
		return Math.min(getContainerWidth() - chartPadding.right, Math.max(dragStartX, dragEndX));
	});
	const selectionWidth = $derived(selectionRight() - selectionLeft);
	const selectionStartTime = $derived(() => getTimeAtPosition(selectionLeft));
	const selectionEndTime = $derived(() => getTimeAtPosition(selectionLeft + selectionWidth));

	function handleMouseMove(e: MouseEvent) {
		if (!containerRef) return;
		const rect = containerRef.getBoundingClientRect();
		mouseX = e.clientX - rect.left;
		mouseY = e.clientY - rect.top;

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
		// Cancel drag if mouse leaves
		if (isDragging) {
			isDragging = false;
		}
		onHoverTimeChange?.(null);
	}

	function handleMouseDown(e: MouseEvent) {
		if (!containerRef || e.button !== 0) return; // Only left click
		const rect = containerRef.getBoundingClientRect();
		const x = e.clientX - rect.left;

		// Only start drag if within chart area
		if (x < chartPadding.left || x > rect.width - chartPadding.right) return;

		isDragging = true;
		dragStartX = x;
		dragEndX = x;

		// Prevent text selection during drag
		e.preventDefault();
	}

	function handleMouseUp(e: MouseEvent) {
		if (!isDragging || !containerRef) return;

		isDragging = false;

		// Only trigger if selection is meaningful (at least 10px)
		if (selectionWidth > 10 && onRangeSelect) {
			const startTime = getTimeAtPosition(Math.min(dragStartX, dragEndX));
			const endTime = getTimeAtPosition(Math.max(dragStartX, dragEndX));
			onRangeSelect(startTime, endTime);
		}
	}

	// Format time for display
	function formatTime(date: Date | null): string {
		if (!date) return '';
		return formatDateTime(date, { timezone: tz, format: 'time' });
	}

	// Handle ESC key to cancel drag selection
	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape' && isDragging) {
			isDragging = false;
			dragStartX = 0;
			dragEndX = 0;
		}
	}

	// Register/cleanup keyboard listener when dragging
	$effect(() => {
		if (isDragging) {
			window.addEventListener('keydown', handleKeydown);
			return () => {
				window.removeEventListener('keydown', handleKeydown);
			};
		}
	});
</script>

<div
	bind:this={containerRef}
	class="relative select-none"
	onmouseenter={handleMouseEnter}
	onmouseleave={handleMouseLeave}
	onmousemove={handleMouseMove}
	onmousedown={handleMouseDown}
	onmouseup={handleMouseUp}
	role="application"
	aria-label="Chart with drag-to-zoom"
	style="cursor: {isDragging ? 'col-resize' : (isInChartArea() ? 'crosshair' : 'default')};"
>
	{@render children()}

	{#if isDragging && selectionWidth > 0}
		<!-- Selection region overlay -->
		<div
			class="absolute top-0 bottom-0 bg-primary/20 border-x border-primary/40 pointer-events-none"
			style="left: {selectionLeft}px; width: {selectionWidth}px;"
		>
			<!-- Selection time labels -->
			<div class="absolute -top-5 left-0 -translate-x-full text-[9px] font-medium text-primary whitespace-nowrap">
				{formatTime(selectionStartTime())}
			</div>
			<div class="absolute -top-5 right-0 translate-x-full text-[9px] font-medium text-primary whitespace-nowrap">
				{formatTime(selectionEndTime())}
			</div>
		</div>
	{/if}

	{#if isHovering && !isDragging && isInChartArea()}
		{@const clampedX = Math.max(chartPadding.left, Math.min(mouseX, getContainerWidth() - chartPadding.right))}
		{@const valueData = calculatedValue()}

		<!-- Vertical line at mouse X position (clamped to chart area) -->
		<div
			class="absolute top-0 bottom-0 w-px bg-muted-foreground/50 pointer-events-none"
			style="left: {clampedX}px;"
		></div>

		<!-- Value tooltip at top -->
		{#if hasMultiServerData && calculatedServerValues().length > 0}
			<!-- Multi-server tooltip -->
			<div
				class="absolute top-0 -translate-x-1/2 -translate-y-full pointer-events-none"
				style="left: {clampedX}px;"
			>
				<div
					class="bg-foreground text-background rounded px-2 py-1 text-xs font-medium whitespace-nowrap shadow-lg mb-1 flex flex-col gap-0.5"
				>
					{#each calculatedServerValues() as serverValue}
						<div class="flex items-center gap-1.5">
							<span
								class="h-2 w-2 rounded-full flex-shrink-0"
								style="background-color: {serverValue.color};"
							></span>
							<span>{formatDisplayValue(serverValue.value)}{#if unit}<span class="text-background/70 ml-0.5">{unit}</span>{/if}</span>
						</div>
					{/each}
				</div>
			</div>
		{:else if valueData}
			<!-- Single-server tooltip -->
			<div
				class="absolute top-0 -translate-x-1/2 -translate-y-full pointer-events-none"
				style="left: {clampedX}px;"
			>
				<div
					class="bg-foreground text-background rounded px-2 py-1 text-xs font-medium whitespace-nowrap shadow-lg mb-1"
				>
					{formatDisplayValue(valueData.value)}{#if unit}<span class="text-background/70 ml-0.5">{unit}</span>{/if}
				</div>
			</div>
		{/if}

		<!-- Time label at bottom, positioned at mouse X -->
		<div
			class="absolute bottom-0 -translate-x-1/2 translate-y-full pointer-events-none"
			style="left: {clampedX}px;"
		>
			<div
				class="bg-background border border-border rounded px-1.5 py-0.5 text-[10px] text-muted-foreground whitespace-nowrap shadow-sm mt-1"
			>
				{formatTime(calculatedTime())}
			</div>
		</div>
	{/if}

	<!-- Shadow tooltip line: shows when another chart is being hovered -->
	{#if shouldShowShadowLine}
		{@const shadowX = shadowLineX()}
		{#if shadowX >= chartPadding.left && shadowX <= getContainerWidth() - chartPadding.right}
			<div
				class="absolute top-0 bottom-0 w-px bg-muted-foreground/25 pointer-events-none"
				style="left: {shadowX}px;"
			></div>
		{/if}
	{/if}
</div>
