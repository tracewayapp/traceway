<script lang="ts">
	import { DateTime } from 'luxon';
	import { Button } from '$lib/components/ui/button';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { ChevronLeft, ChevronRight } from '@lucide/svelte';
	import {
		oncallState,
		type Schedule,
		type TimelineResponse,
		type TimelineShift
	} from '$lib/state/oncall.svelte';
	import { getTimezone } from '$lib/state/timezone.svelte';
	import { formatDateTime } from '$lib/utils/formatters';

	interface Props {
		organizationId: number;
		scheduleId: number;
		schedule?: Schedule | null;
	}

	let { organizationId, scheduleId, schedule = null }: Props = $props();

	type Preset = 'day' | 'week' | '2weeks' | 'month';

	const presets: { value: Preset; label: string }[] = [
		{ value: 'day', label: 'Day' },
		{ value: 'week', label: 'Week' },
		{ value: '2weeks', label: '2 Weeks' },
		{ value: 'month', label: 'Month' }
	];

	let preset = $state<Preset>('week');
	let anchorISO = $state<string | null>(null);
	let timeline = $state<TimelineResponse | null>(null);
	let loading = $state(true);
	let error = $state('');

	const tz = $derived(timeline?.schedule.timezone || schedule?.timezone || getTimezone());

	function presetDuration(p: Preset) {
		switch (p) {
			case 'day':
				return { days: 1 };
			case 'week':
				return { weeks: 1 };
			case '2weeks':
				return { weeks: 2 };
			case 'month':
				return { months: 1 };
		}
	}

	function defaultAnchor(p: Preset): DateTime {
		const now = DateTime.now().setZone(tz);
		if (p === 'day') return now.startOf('day');
		if (p === 'month') return now.startOf('month');
		return now.startOf('week');
	}

	const anchor = $derived.by(() => {
		if (anchorISO) {
			const dt = DateTime.fromISO(anchorISO, { zone: tz });
			if (dt.isValid) return dt;
		}
		return defaultAnchor(preset);
	});

	const rangeFrom = $derived(anchor);
	const rangeTo = $derived(anchor.plus(presetDuration(preset)));

	function setPreset(p: Preset) {
		preset = p;
		anchorISO = null;
	}

	function shiftRange(direction: -1 | 1) {
		const dur = presetDuration(preset);
		anchorISO = (direction === 1 ? anchor.plus(dur) : anchor.minus(dur)).toISO();
	}

	function goToday() {
		anchorISO = null;
	}

	let loadSeq = 0;

	$effect(() => {
		const from = rangeFrom.toUTC().toISO();
		const to = rangeTo.toUTC().toISO();
		if (!from || !to) return;
		const seq = ++loadSeq;
		loading = true;
		error = '';
		oncallState
			.getTimeline(organizationId, scheduleId, from, to)
			.then((res) => {
				if (seq !== loadSeq) return;
				timeline = res;
				loading = false;
			})
			.catch((e: unknown) => {
				if (seq !== loadSeq) return;
				error = e instanceof Error ? e.message : 'Failed to load timeline';
				loading = false;
			});
	});

	const totalMs = $derived(rangeTo.toMillis() - rangeFrom.toMillis());

	const rows = $derived.by(() => {
		if (!timeline) return [];
		return [
			...timeline.layers.map((l) => ({ id: l.id, name: l.name, shifts: l.shifts })),
			{ id: '__final__', name: 'Schedule', shifts: timeline.final }
		];
	});

	const ticks = $derived.by(() => {
		const result: { pos: number; label: string }[] = [];
		const step = preset === 'day' ? { hours: 3 } : preset === 'month' ? { days: 2 } : { days: 1 };
		let cursor = rangeFrom;
		while (cursor < rangeTo) {
			const pos = ((cursor.toMillis() - rangeFrom.toMillis()) / totalMs) * 100;
			const label =
				preset === 'day'
					? cursor.toFormat('HH:mm')
					: preset === 'month'
						? cursor.toFormat('d')
						: cursor.toFormat('ccc d');
			result.push({ pos, label });
			cursor = cursor.plus(step);
		}
		return result;
	});

	const nowPos = $derived.by(() => {
		const now = Date.now();
		if (now < rangeFrom.toMillis() || now > rangeTo.toMillis()) return null;
		return ((now - rangeFrom.toMillis()) / totalMs) * 100;
	});

	const palette = [
		'#3b82f6',
		'#10b981',
		'#f59e0b',
		'#8b5cf6',
		'#ec4899',
		'#06b6d4',
		'#84cc16',
		'#f97316'
	];

	function userColor(userId: number): string {
		return palette[((userId * 2654435761) >>> 0) % palette.length];
	}

	function isFormerMember(userId: number): boolean {
		return timeline?.users?.[String(userId)] === null;
	}

	function userName(userId: number): string {
		const user = timeline?.users?.[String(userId)];
		if (user === null) return 'Former member';
		return user?.name || user?.email || `User #${userId}`;
	}

	interface PositionedShift {
		shift: TimelineShift;
		left: number;
		width: number;
	}

	function positionShifts(shifts: TimelineShift[]): PositionedShift[] {
		const fromMs = rangeFrom.toMillis();
		const toMs = rangeTo.toMillis();
		const result: PositionedShift[] = [];
		for (const shift of shifts) {
			const start = Math.max(new Date(shift.start).getTime(), fromMs);
			const end = Math.min(new Date(shift.end).getTime(), toMs);
			if (end <= start) continue;
			result.push({
				shift,
				left: ((start - fromMs) / totalMs) * 100,
				width: ((end - start) / totalMs) * 100
			});
		}
		return result;
	}

	// The tooltip reads in the schedule's timezone, matching the axis above it.
	function shiftTooltip(shift: TimelineShift): string {
		const range = `${formatDateTime(shift.start, { format: 'short', timezone: tz })} – ${formatDateTime(shift.end, { format: 'short', timezone: tz })}`;
		return `${userName(shift.userId)}${shift.isOverride ? ' (override)' : ''}\n${range} (${tz})`;
	}

	const rangeLabel = $derived.by(() => {
		if (preset === 'day') return rangeFrom.toFormat('ccc, MMM d yyyy');
		const end = rangeTo.minus({ days: 1 });
		return `${rangeFrom.toFormat('MMM d')} – ${end.toFormat('MMM d yyyy')}`;
	});
</script>

<div class="space-y-2">
	<div class="flex flex-wrap items-center justify-between gap-2">
		<div class="flex items-center gap-1">
			{#each presets as p (p.value)}
				<Button
					variant={preset === p.value ? 'secondary' : 'ghost'}
					size="sm"
					onclick={() => setPreset(p.value)}
				>
					{p.label}
				</Button>
			{/each}
		</div>
		<div class="flex items-center gap-1">
			<Button variant="ghost" size="icon" title="Previous" onclick={() => shiftRange(-1)}>
				<ChevronLeft class="h-4 w-4" />
			</Button>
			<span class="min-w-40 text-center text-sm text-muted-foreground">{rangeLabel}</span>
			<Button variant="ghost" size="icon" title="Next" onclick={() => shiftRange(1)}>
				<ChevronRight class="h-4 w-4" />
			</Button>
			<Button variant="outline" size="sm" onclick={goToday}>Today</Button>
		</div>
	</div>

	{#if loading && !timeline}
		<div class="flex justify-center py-12"><LoadingCircle size="xlg" /></div>
	{:else if error}
		<div class="rounded-md border py-8 text-center text-sm text-destructive">{error}</div>
	{:else if timeline}
		<div class="overflow-x-auto rounded-md border">
			<div class="grid min-w-[640px] grid-cols-[140px_1fr]" class:opacity-60={loading}>
				<div class="border-r border-b px-3 py-1.5 text-xs text-muted-foreground">
					{tz}
				</div>
				<div class="relative h-7 border-b">
					{#each ticks as tick (tick.pos)}
						<span
							class="absolute top-1.5 pl-1 text-[10px] whitespace-nowrap text-muted-foreground"
							style="left: {tick.pos}%"
						>
							{tick.label}
						</span>
					{/each}
				</div>

				{#each rows as row (row.id)}
					<div
						class="flex items-center border-r border-b px-3 py-2 text-sm {row.id === '__final__'
							? 'font-medium'
							: ''}"
					>
						<span class="truncate">{row.name}</span>
					</div>
					<div class="relative h-12 border-b bg-muted/40">
						{#each ticks as tick (tick.pos)}
							<div
								class="absolute inset-y-0 border-l border-border/40"
								style="left: {tick.pos}%"
							></div>
						{/each}
						{#each positionShifts(row.shifts) as { shift, left, width }, i (i)}
							<div
								class="absolute inset-y-1.5 flex items-center overflow-hidden rounded px-1.5 text-[11px] font-medium text-white {shift.isOverride
									? 'border-2 border-dashed border-white/80'
									: ''}"
								style="left: {left}%; width: {width}%; background-color: {isFormerMember(
									shift.userId
								)
									? 'var(--destructive)'
									: userColor(shift.userId)}; {shift.isOverride
									? 'background-image: repeating-linear-gradient(45deg, rgba(255,255,255,0.18) 0, rgba(255,255,255,0.18) 6px, transparent 6px, transparent 12px);'
									: ''}"
								title={shiftTooltip(shift)}
							>
								{#if width > 6}
									<span class="truncate {isFormerMember(shift.userId) ? 'italic' : ''}">
										{userName(shift.userId)}
									</span>
									{#if shift.isOverride && width > 14}
										<span class="ml-1.5 shrink-0 rounded bg-white/25 px-1 text-[9px] uppercase">
											Override
										</span>
									{/if}
								{/if}
							</div>
						{/each}
						{#if nowPos !== null}
							<div
								class="pointer-events-none absolute inset-y-0 z-10 w-0.5 bg-red-500"
								style="left: {nowPos}%"
							></div>
						{/if}
					</div>
				{/each}
			</div>
		</div>
		{#if timeline.layers.length === 0}
			<p class="text-sm text-muted-foreground">
				This schedule has no layers yet. Add layers to generate shifts.
			</p>
		{/if}
	{/if}
</div>
