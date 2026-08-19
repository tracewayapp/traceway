<script lang="ts">
	import * as Tooltip from '$lib/components/ui/tooltip';
	import { formatDateTime, getNow } from '$lib/utils/formatters';
	import { getTimezone } from '$lib/state/timezone.svelte';

	type TrendPoint = { timestamp: string; count: number };

	let {
		trend = [],
		timezone
	}: {
		trend: TrendPoint[];
		timezone?: string;
	} = $props();

	const tz = $derived(timezone ?? getTimezone());

	// Always 24 hours
	const HOURS = 24;
	const SCALE_MAX = 10; // Visual scale max - bars are relative to this

	// Build 24-hour data array, filling gaps with 0
	const hourlyData = $derived(() => {
		const nowDt = getNow(tz);
		const result: { hour: Date; count: number }[] = [];

		// Create a map of hour -> count from trend data
		const trendMap = new Map<string, number>();
		for (const point of trend) {
			const date = new Date(point.timestamp);
			const key = `${date.getFullYear()}-${date.getMonth()}-${date.getDate()}-${date.getHours()}`;
			trendMap.set(key, (trendMap.get(key) || 0) + point.count);
		}

		// Generate last 24 hours
		for (let i = HOURS - 1; i >= 0; i--) {
			const hourDt = nowDt.minus({ hours: i }).startOf('hour');
			const hour = hourDt.toJSDate();
			const key = `${hour.getFullYear()}-${hour.getMonth()}-${hour.getDate()}-${hour.getHours()}`;
			result.push({
				hour,
				count: trendMap.get(key) || 0
			});
		}

		return result;
	});

	// Actual max from data
	const actualMax = $derived(() => {
		return Math.max(...hourlyData().map((d) => d.count), 0);
	});

	// Scale max - at least SCALE_MAX, or higher if data exceeds it
	const scaleMax = $derived(() => {
		return Math.max(SCALE_MAX, actualMax());
	});

	// Dashed line position as percentage from bottom (actual max relative to scale max)
	const linePositionPct = $derived(() => {
		if (actualMax() === 0) return 0;
		return (actualMax() / scaleMax()) * 100;
	});
</script>

<div class="relative h-8 w-44 flex-shrink-0">
	<!-- Solid bottom line -->
	<div class="absolute right-7 bottom-0 left-0 border-t border-muted-foreground/30"></div>

	<!-- Dashed line at actual max -->
	{#if actualMax() > 0}
		<div
			class="absolute right-7 left-0 h-[1px]"
			style="bottom: {linePositionPct()}%; background: repeating-linear-gradient(to right, var(--muted-foreground) 0, var(--muted-foreground) 4px, transparent 4px, transparent 7px); opacity: 0.4;"
		></div>
		<!-- Max count label -->
		<span
			class="absolute right-1 w-[20px] text-[10px] font-medium text-muted-foreground tabular-nums"
			style="bottom: calc({linePositionPct()}% - 6px);"
		>
			{actualMax() > 999 ? (actualMax() / 1000).toFixed(1) + 'k' : actualMax()}
		</span>
	{/if}

	<!-- Bars container -->
	<div class="absolute inset-0 right-7 flex items-end gap-[2px]">
		{#each hourlyData() as point, i (i)}
			{@const heightPct = scaleMax() > 0 ? (point.count / scaleMax()) * 100 : 0}
			{@const hasEvents = point.count > 0}
			{@const isPeak = hasEvents && point.count === actualMax()}
			<Tooltip.Root>
				<Tooltip.Trigger class="flex h-full flex-1 items-end justify-center">
					<div
						class="w-full max-w-[5px] rounded-[2px] {hasEvents
							? isPeak
								? 'bg-pink-500 hover:opacity-80 dark:bg-pink-400'
								: 'bg-(--trend-bar) hover:opacity-80'
							: 'bg-foreground/10'}"
						style="height: {Math.max(hasEvents ? 8 : 3, heightPct)}%;"
					></div>
				</Tooltip.Trigger>
				<Tooltip.Content side="top" class="!animate-none px-3 py-2 !transition-none">
					<div class="text-sm font-medium">
						{point.count}
						{point.count === 1 ? 'Event' : 'Events'}
					</div>
					<div class="text-xs text-muted-foreground">
						{formatDateTime(point.hour, { timezone: tz, format: 'full' })}
					</div>
				</Tooltip.Content>
			</Tooltip.Root>
		{/each}
	</div>
</div>
