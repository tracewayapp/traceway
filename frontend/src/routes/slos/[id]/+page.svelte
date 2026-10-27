<script lang="ts">
	import { getErrorMessage, getErrorStatus } from '$lib/utils/errors';
	import { onMount, onDestroy } from 'svelte';
	import { api } from '$lib/api';
	import {
		formatDateTime,
		formatRelativeTimeAgo,
		toUTCISO,
		calendarDateTimeToLuxon
	} from '$lib/utils/formatters';
	import * as Card from '$lib/components/ui/card';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { ErrorDisplay } from '$lib/components/ui/error-display';
	import { TimeRangePicker } from '$lib/components/ui/time-range-picker';
	import SloKindBadge from '$lib/components/slos/slo-kind-badge.svelte';
	import SloStatusBadge from '$lib/components/slos/slo-status-badge.svelte';
	import SloSheet from '../slo-sheet.svelte';
	import D3LineChart from '$lib/components/dashboard/d3-line-chart.svelte';
	import PageHeader from '$lib/components/traceway/page-header.svelte';
	import StatRow from '$lib/components/traceway/stat-row.svelte';
	import StatTile from '$lib/components/traceway/stat-tile.svelte';
	import StatusPill from '$lib/components/traceway/status-pill.svelte';
	import ChartCard from '$lib/components/traceway/chart-card.svelte';
	import InfoCallout from '$lib/components/traceway/info-callout.svelte';
	import ConfirmDeleteDialog from '$lib/components/traceway/confirm-delete-dialog.svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { CalendarDate } from '@internationalized/date';
	import { projectsState } from '$lib/state/projects.svelte';
	import { getTimezone } from '$lib/state/timezone.svelte';
	import {
		slosState,
		type SLO,
		type SloAlertRule,
		type SloSeriesBucket
	} from '$lib/state/slos.svelte';
	import { createSmartBackHandler } from '$lib/utils/back-navigation';
	import {
		parseTimeRangeFromUrl,
		getResolvedTimeRange,
		dateToCalendarDate,
		dateToTimeString,
		updateUrl
	} from '$lib/utils/url-params';
	import {
		budgetTone,
		burnTone,
		calculateSloInterval,
		clampRangeToWindow,
		defaultPresetForWindow,
		errorBudgetSeries,
		formatBudgetRemaining,
		formatBurnRate,
		formatSloPercent,
		SLO_BUCKET_RETENTION_DAYS,
		sliChartDomain,
		budgetChartDomain,
		sliSeries,
		sliSummary,
		toneTextClass
	} from '$lib/utils/slo-math';
	import { badEventsHref, badEventsLabel } from '$lib/utils/slo-links';
	import { ruleTypeLabels } from '../../notifications/rule-types';
	import { Bell, ChartLine, ExternalLink, Pencil, PiggyBank, Trash2 } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';

	let { data } = $props();

	// svelte-ignore state_referenced_locally
	const sloId = data.id;
	// svelte-ignore state_referenced_locally
	const urlHasRange = !!(data.preset || data.from || data.to);
	const initialTimezone = getTimezone();
	const timezone = $derived(getTimezone());
	const initialUrlParams = parseTimeRangeFromUrl(initialTimezone, '1M');
	const initialRange = getResolvedTimeRange(initialUrlParams, initialTimezone);

	let slo = $state<SLO | null>(null);
	let rules = $state<SloAlertRule[]>([]);
	let loading = $state(true);
	let error = $state('');
	let notFound = $state(false);

	let buckets = $state<SloSeriesBucket[]>([]);
	let seriesLoading = $state(true);
	let seriesError = $state('');
	let seriesSequence = 0;
	let rangeFrom = $state(initialRange.from);
	let rangeTo = $state(initialRange.to);

	let selectedPreset = $state<string | null>(initialUrlParams.preset);
	let fromDate = $state<CalendarDate>(dateToCalendarDate(initialRange.from, initialTimezone));
	let toDate = $state<CalendarDate>(dateToCalendarDate(initialRange.to, initialTimezone));
	let fromTime = $state(dateToTimeString(initialRange.from, initialTimezone));
	let toTime = $state(dateToTimeString(initialRange.to, initialTimezone));

	let editOpen = $state(false);
	let deleteOpen = $state(false);
	let deleting = $state(false);

	function getFromDateTimeUTC(): string {
		const [hour, minute] = (fromTime || '00:00').split(':').map(Number);
		return toUTCISO(
			calendarDateTimeToLuxon(
				{ year: fromDate.year, month: fromDate.month, day: fromDate.day, hour, minute },
				timezone
			)
		);
	}

	function getToDateTimeUTC(): string {
		const [hour, minute] = (toTime || '23:59').split(':').map(Number);
		return toUTCISO(
			calendarDateTimeToLuxon(
				{ year: toDate.year, month: toDate.month, day: toDate.day, hour, minute },
				timezone
			).endOf('minute')
		);
	}

	function applyRange(from: Date, to: Date) {
		fromDate = dateToCalendarDate(from, timezone);
		fromTime = dateToTimeString(from, timezone);
		toDate = dateToCalendarDate(to, timezone);
		toTime = dateToTimeString(to, timezone);
	}

	function resolveRange(): { from: Date; to: Date } {
		if (selectedPreset) {
			return getResolvedTimeRange({ preset: selectedPreset, from: null, to: null }, timezone);
		}
		return { from: new Date(getFromDateTimeUTC()), to: new Date(getToDateTimeUTC()) };
	}

	async function loadSlo() {
		try {
			const response = await api.get(`/slos/${sloId}`, {
				projectId: projectsState.currentProjectId ?? undefined
			});
			slo = response.slo;
			rules = response.rules || [];
		} catch (e) {
			if (getErrorStatus(e) === 404) {
				notFound = true;
			} else {
				error = getErrorMessage(e) || 'Failed to load the SLO';
			}
		}
	}

	async function loadSeries(pushToHistory: boolean, announceClamp: boolean) {
		if (!slo) return;
		const sequence = ++seriesSequence;
		seriesLoading = true;
		const picked = resolveRange();
		let clamped = clampRangeToWindow(picked.from, picked.to, slo.windowDays, new Date());
		if (clamped.clamped) {
			if (selectedPreset) {
				selectedPreset = defaultPresetForWindow(slo.windowDays);
				clamped = { ...resolveRange(), clamped: true };
			}
			applyRange(clamped.from, clamped.to);
			if (announceClamp) {
				toast.info(`Range limited to the SLO's ${slo.windowDays}-day window`);
			}
		}
		rangeFrom = clamped.from;
		rangeTo = clamped.to;
		updateUrl(
			selectedPreset
				? { preset: selectedPreset, from: null, to: null }
				: { preset: null, from: rangeFrom.toISOString(), to: rangeTo.toISOString() },
			{ pushToHistory }
		);
		try {
			const response = await api.post(
				`/slos/${sloId}/series`,
				{
					fromDate: rangeFrom.toISOString(),
					toDate: rangeTo.toISOString(),
					intervalMinutes: calculateSloInterval(rangeFrom, rangeTo, slo.bucketMinutes)
				},
				{ projectId: projectsState.currentProjectId ?? undefined }
			);
			if (sequence !== seriesSequence) return;
			buckets = response.buckets || [];
			seriesError = '';
		} catch (e) {
			if (sequence !== seriesSequence) return;
			buckets = [];
			seriesError = getErrorMessage(e) || 'Failed to load the history of this SLO';
		} finally {
			if (sequence === seriesSequence) seriesLoading = false;
		}
	}

	async function reloadAfterSave() {
		await loadSlo();
		if (slo) loadSeries(false, false);
	}

	async function loadAll() {
		loading = true;
		error = '';
		notFound = false;
		await loadSlo();
		loading = false;
		if (slo && !urlHasRange && selectedPreset === initialUrlParams.preset) {
			selectedPreset = defaultPresetForWindow(slo.windowDays);
		}
		if (!notFound && !error) {
			loadSeries(false, false);
		}
	}

	function handleTimeRangeChange(
		from: { date: CalendarDate; time: string },
		to: { date: CalendarDate; time: string },
		preset: string | null
	) {
		fromDate = from.date;
		fromTime = from.time;
		toDate = to.date;
		toTime = to.time;
		selectedPreset = preset;
		loadSeries(true, true);
	}

	function handlePopState() {
		const urlParams = parseTimeRangeFromUrl(
			timezone,
			slo ? defaultPresetForWindow(slo.windowDays) : '1M'
		);
		const range = getResolvedTimeRange(urlParams, timezone);
		selectedPreset = urlParams.preset;
		applyRange(range.from, range.to);
		loadSeries(false, false);
	}

	async function deleteSlo() {
		if (!slo) return;
		deleting = true;
		try {
			await api.delete(`/slos/${sloId}`, {
				projectId: projectsState.currentProjectId ?? undefined
			});
			toast.success('Successfully deleted the SLO');
			slosState.refreshBreachedCount();
			goto(resolve('/slos'));
		} catch (e) {
			if (getErrorStatus(e) !== 403) {
				toast.error(getErrorMessage(e) || 'Failed to delete the SLO');
			}
		} finally {
			deleting = false;
			deleteOpen = false;
		}
	}

	const sliPoints = $derived(sliSeries(buckets));
	const budgetPoints = $derived(slo ? errorBudgetSeries(buckets, slo.targetPercent) : []);
	const sliChartSeries = $derived([{ key: 'SLI', color: 'var(--chart-1)', data: sliPoints }]);
	const budgetChartSeries = $derived([
		{ key: 'Budget consumed', color: 'var(--chart-2)', data: budgetPoints }
	]);
	const coverage = $derived.by(() => {
		if (!slo?.historyFrom) return null;
		const historyFrom = new Date(
			Math.max(
				new Date(slo.historyFrom).getTime(),
				Date.now() - SLO_BUCKET_RETENTION_DAYS * 86400000
			)
		);
		if (historyFrom.getTime() <= rangeFrom.getTime()) return null;
		const coveredDays = Math.max(0, (rangeTo.getTime() - historyFrom.getTime()) / 86400000);
		const rangeDays = (rangeTo.getTime() - rangeFrom.getTime()) / 86400000;
		return { historyFrom, coveredDays, rangeDays };
	});

	function formatDays(days: number): string {
		return days >= 10 ? days.toFixed(0) : days.toFixed(1);
	}

	function ruleStateLabel(rule: SloAlertRule): {
		tone: 'success' | 'warning' | 'neutral';
		label: string;
	} {
		if (!rule.enabled) return { tone: 'neutral', label: 'Off' };
		if (rule.snoozedUntil && new Date(rule.snoozedUntil).getTime() > Date.now()) {
			return { tone: 'warning', label: 'Snoozed' };
		}
		return { tone: 'success', label: 'On' };
	}

	onMount(() => {
		window.addEventListener('popstate', handlePopState);
		loadAll();
	});

	onDestroy(() => {
		if (typeof window !== 'undefined') {
			window.removeEventListener('popstate', handlePopState);
		}
	});
</script>

{#if loading}
	<div class="flex h-64 items-center justify-center">
		<LoadingCircle size="xlg" />
	</div>
{:else if notFound}
	<ErrorDisplay
		status={404}
		title="SLO not found"
		description="This SLO does not exist in the current project."
		onBack={createSmartBackHandler({ fallbackPath: '/slos' })}
		backLabel="Back to SLOs"
	/>
{:else if error || !slo}
	<ErrorDisplay
		status={400}
		title="Error"
		description={error || 'Failed to load the SLO'}
		onRetry={() => loadAll()}
	/>
{:else}
	{@const budget = budgetTone(slo.budgetRemaining)}
	<div class="space-y-4">
		<PageHeader
			title={slo.name}
			description={slo.description || undefined}
			onBack={createSmartBackHandler({ fallbackPath: '/slos' })}
		>
			{#snippet meta()}
				{#if slo}
					<SloStatusBadge status={slo.status} enabled={slo.enabled} />
					<SloKindBadge kind={slo.sliKind} />
					<span class="text-sm text-muted-foreground">Target {slo.targetPercent}%</span>
					<span class="text-sm text-muted-foreground">{slo.windowDays}-day window</span>
					<span class="min-w-0 font-mono text-sm break-all text-muted-foreground"
						>{sliSummary(slo)}</span
					>
					{#if slo.computedAt}
						<span class="text-sm text-muted-foreground">
							updated {formatRelativeTimeAgo(slo.computedAt)}
						</span>
					{/if}
				{/if}
			{/snippet}
			{#snippet actions()}
				<TimeRangePicker
					bind:fromDate
					bind:toDate
					bind:fromTime
					bind:toTime
					bind:preset={selectedPreset}
					onApply={handleTimeRangeChange}
				/>
				{#if slo && projectsState.canWriteCurrentProject}
					<Button variant="outline" size="sm" onclick={() => (editOpen = true)}>
						<Pencil class="mr-2 h-4 w-4" />
						Edit
					</Button>
					<Button variant="destructiveOutline" size="sm" onclick={() => (deleteOpen = true)}>
						<Trash2 class="mr-2 h-4 w-4" />
						Delete
					</Button>
				{/if}
			{/snippet}
		</PageHeader>

		<StatRow columns={4}>
			<StatTile
				label="Current SLI"
				value={formatSloPercent(slo.currentSli, slo.targetPercent)}
				valueClass={toneTextClass(budget)}
				sub="target {slo.targetPercent}% over {slo.windowDays} days"
			/>
			<StatTile
				label="Error budget left"
				value={formatBudgetRemaining(slo.budgetRemaining)}
				valueClass={toneTextClass(budget)}
				sub={slo.budgetRemaining !== null && slo.budgetRemaining < 0
					? 'overspent: more failures than the target allows'
					: 'share of the allowed failures not yet used'}
			/>
			<StatTile
				label="Burn rate 1h"
				value={formatBurnRate(slo.burnRate1h)}
				valueClass={toneTextClass(burnTone(slo.burnRate1h))}
				sub="1x spends the budget exactly over the window"
			/>
			<StatTile
				label="Burn rate 6h"
				value={formatBurnRate(slo.burnRate6h)}
				valueClass={toneTextClass(burnTone(slo.burnRate6h))}
				sub="sustained burn over the last six hours"
			/>
		</StatRow>

		{#if coverage}
			<InfoCallout>
				History covers {formatDays(coverage.coveredDays)} of {formatDays(coverage.rangeDays)} days in
				this range. Buckets before {formatDateTime(coverage.historyFrom.toISOString(), {
					format: 'short'
				})} are still being computed or fall outside telemetry retention.
			</InfoCallout>
		{/if}

		<ChartCard
			title="SLI"
			loading={seriesLoading}
			empty={sliPoints.length === 0}
			emptyMessage={seriesError || 'No events in this range'}
		>
			{#snippet icon()}
				<ChartLine class="h-3.5 w-3.5 text-muted-foreground" />
			{/snippet}
			<D3LineChart
				series={sliChartSeries}
				referenceLines={[{ value: slo.targetPercent, label: `Target ${slo.targetPercent}%` }]}
				yDomain={sliChartDomain(sliPoints, slo.targetPercent)}
				formatValue={(value) => `${value.toFixed(2)}%`}
			/>
		</ChartCard>

		<ChartCard
			title="Error budget consumed"
			loading={seriesLoading}
			empty={budgetPoints.length === 0}
			emptyMessage={seriesError || 'No events in this range'}
		>
			{#snippet icon()}
				<PiggyBank class="h-3.5 w-3.5 text-muted-foreground" />
			{/snippet}
			<D3LineChart
				series={budgetChartSeries}
				referenceLines={[{ value: 100, label: 'Budget exhausted' }]}
				yDomain={budgetChartDomain(budgetPoints)}
				formatValue={(value) => `${value.toFixed(1)}%`}
				areaFill
			/>
			<p class="mt-2 text-xs text-muted-foreground">
				Cumulative share of the error budget the traffic in this range has used. On the SLO's own
				window it ends at 100% minus the budget left.
			</p>
		</ChartCard>

		<Card.Root class="gap-3 pt-2">
			<Card.Header
				class="flex flex-row flex-wrap items-center justify-between gap-3 space-y-0 border-b pl-3 [.border-b]:pb-2"
			>
				<Card.Title class="flex items-center gap-2 text-base font-medium">
					<Bell class="h-3.5 w-3.5 text-muted-foreground" />
					<span class="text-sm font-[400] text-muted-foreground">Alert rules</span>
				</Card.Title>
				<div class="flex items-center gap-2">
					<Button variant="outline" size="sm" href="/notifications?tab=rules">
						Manage alert rules
					</Button>
					<Button variant="outline" size="sm" href={badEventsHref(slo)}>
						<ExternalLink class="mr-2 h-3.5 w-3.5" />
						{badEventsLabel(slo)}
					</Button>
				</div>
			</Card.Header>
			<Card.Content class="pb-4">
				{#if rules.length === 0}
					<p class="text-sm text-muted-foreground">
						No alert rules reference this SLO yet. Add an SLO Burn Rate or SLO Budget Consumed rule
						on the Alerts page to get paged when the budget burns too fast.
					</p>
				{:else}
					<div class="space-y-2">
						{#each rules as rule (rule.id)}
							{@const state = ruleStateLabel(rule)}
							<div
								class="flex flex-col gap-2 rounded-md border p-3 text-sm sm:flex-row sm:items-center sm:justify-between sm:gap-4"
							>
								<div class="min-w-0">
									<div class="font-medium">{rule.name}</div>
									<div class="text-xs text-muted-foreground">
										{ruleTypeLabels[rule.ruleType] ?? rule.ruleType}
										{#if rule.ruleType === 'slo_burn_rate'}
											· {rule.config.burnRateThreshold ?? 14.4}x over {rule.config
												.longWindowMinutes ?? 60}m /
											{rule.config.shortWindowMinutes ?? 5}m
										{:else if rule.ruleType === 'slo_budget_consumed'}
											· {rule.config.consumedPercent ?? 80}% consumed
										{/if}
									</div>
								</div>
								<div class="flex shrink-0 items-center gap-2">
									<Badge variant="outline">{rule.channelName}</Badge>
									<StatusPill tone={state.tone} label={state.label} />
								</div>
							</div>
						{/each}
					</div>
				{/if}
			</Card.Content>
		</Card.Root>
	</div>

	<SloSheet bind:open={editOpen} {slo} onSaved={reloadAfterSave} />

	<ConfirmDeleteDialog
		bind:open={deleteOpen}
		entity="SLO"
		description={`This permanently deletes "${slo.name}" and its computed history. Alert rules that reference it are removed too.`}
		loading={deleting}
		onConfirm={deleteSlo}
	/>
{/if}
