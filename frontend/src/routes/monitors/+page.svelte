<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { api } from '$lib/api';
	import { toUTCISO, calendarDateTimeToLuxon, formatDurationMs } from '$lib/utils/formatters';
	import { getTimezone } from '$lib/state/timezone.svelte';
	import * as Table from '$lib/components/ui/table';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { TracewayTableHeader } from '$lib/components/ui/traceway-table-header';
	import { TableEmptyState } from '$lib/components/ui/table-empty-state';
	import { TimeRangePicker } from '$lib/components/ui/time-range-picker';
	import { SearchBar } from '$lib/components/ui/search-bar';
	import InfoCallout from '$lib/components/traceway/info-callout.svelte';
	import TabsRow from '$lib/components/traceway/tabs-row.svelte';
	import EmptyState from '$lib/components/traceway/empty-state.svelte';
	import CheckStatusBadge from '$lib/components/synthetics/check-status-badge.svelte';
	import MonitorTypeBadge from '$lib/components/synthetics/monitor-type-badge.svelte';
	import MonitorSheet from './monitor-sheet.svelte';
	import StatusPagesTab from './status-pages-tab.svelte';
	import { browser } from '$app/environment';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { CalendarDate } from '@internationalized/date';
	import { projectsState } from '$lib/state/projects.svelte';
	import { monitorsState, type CheckOverview } from '$lib/state/monitors.svelte';
	import { createRowClickHandler } from '$lib/utils/navigation';
	import { resolve } from '$app/paths';
	import PageHeader from '$lib/components/issues/page-header.svelte';
	import {
		getTimeRangeFromPreset,
		dateToCalendarDate,
		dateToTimeString,
		parseTimeRangeFromUrl,
		getResolvedTimeRange,
		updateUrl
	} from '$lib/utils/url-params';
	import {
		getSortState,
		setSortState,
		handleSortClick,
		type SortDirection
	} from '$lib/utils/sort-storage';

	const timezone = $derived(getTimezone());
	const initialTimezone = getTimezone();

	type SortField = 'name' | 'check_type' | 'status' | 'uptime' | 'latency' | 'last_run';

	let checks = $state<CheckOverview[]>([]);
	let loading = $state(true);
	let error = $state('');

	let dialogOpen = $state(false);

	let statusPagesTab = $state<{ openCreate: () => void } | null>(null);

	const TABS = $derived([
		{ value: 'monitors', label: 'Monitors' },
		...(projectsState.canManageCurrentProject
			? [{ value: 'status-pages', label: 'Status Pages' }]
			: [])
	]);

	const tabDescriptions: Record<string, string> = {
		monitors:
			'Monitors probe your services from the outside on a schedule: an HTTP request, a TCP connect, or a full browser flow. A monitor goes down only after the configured number of consecutive failures.',
		'status-pages':
			'A status page publishes the uptime of selected monitors at /status/<slug> with no login required. Brand it with your title and logo, and optionally serve it from your own domain via a CNAME.'
	};

	const activeTab = $derived.by(() => {
		const tab = page.url.searchParams.get('tab') || 'monitors';
		return TABS.some((t) => t.value === tab) ? tab : 'monitors';
	});

	function setTab(tab: string) {
		const url = new URL(window.location.href);
		if (tab === 'monitors') {
			url.searchParams.delete('tab');
		} else {
			url.searchParams.set('tab', tab);
			url.searchParams.delete('preset');
			url.searchParams.delete('from');
			url.searchParams.delete('to');
			url.searchParams.delete('search');
		}
		goto(url.toString(), { replaceState: true, noScroll: true });
		if (tab === 'monitors') {
			loadData(false);
		}
	}

	const newAction = $derived.by(() => {
		if (activeTab === 'monitors') {
			return projectsState.canWriteCurrentProject
				? { label: 'New Monitor', onclick: () => (dialogOpen = true) }
				: null;
		}
		if (activeTab === 'status-pages') {
			return { label: 'New Status Page', onclick: () => statusPagesTab?.openCreate() };
		}
		return null;
	});

	const typeOptions = [
		{ value: '', label: 'All types' },
		{ value: 'http', label: 'HTTP' },
		{ value: 'tcp', label: 'TCP' },
		{ value: 'browser', label: 'Browser' }
	];
	let typeFilter = $state('');

	function parseMonitorsUrlParams() {
		if (!browser) return { preset: '24h', from: null, to: null, search: '' };
		const params = new URLSearchParams(window.location.search);
		const timeParams = parseTimeRangeFromUrl(timezone, '24h');
		return { ...timeParams, search: params.get('search') || '' };
	}

	const initialUrlParams = parseMonitorsUrlParams();
	const initialRange = getResolvedTimeRange(initialUrlParams, initialTimezone);

	let searchQuery = $state(initialUrlParams.search);

	let selectedPreset = $state<string | null>(initialUrlParams.preset);
	let fromDate = $state<CalendarDate>(dateToCalendarDate(initialRange.from, initialTimezone));
	let toDate = $state<CalendarDate>(dateToCalendarDate(initialRange.to, initialTimezone));
	let fromTime = $state(dateToTimeString(initialRange.from, initialTimezone));
	let toTime = $state(dateToTimeString(initialRange.to, initialTimezone));

	function updateTimeRangeUrl(pushToHistory = true) {
		const params: Record<string, string | null | undefined> = {};
		if (selectedPreset) {
			params.preset = selectedPreset;
		} else {
			params.from = getFromDateTimeUTC();
			params.to = getToDateTimeUTC();
		}
		if (searchQuery.trim()) {
			params.search = searchQuery.trim();
		}
		updateUrl(params, { pushToHistory });
	}

	function handlePopState() {
		const tab = new URLSearchParams(window.location.search).get('tab') || 'monitors';
		if (tab !== 'monitors') return;
		const urlParams = parseMonitorsUrlParams();
		const range = getResolvedTimeRange(urlParams, timezone);

		selectedPreset = urlParams.preset;
		fromDate = dateToCalendarDate(range.from, timezone);
		fromTime = dateToTimeString(range.from, timezone);
		toDate = dateToCalendarDate(range.to, timezone);
		toTime = dateToTimeString(range.to, timezone);
		searchQuery = urlParams.search;

		loadData(false);
	}

	const SORT_STORAGE_KEY = 'monitors';
	const initialSort = getSortState(SORT_STORAGE_KEY, { field: 'name', direction: 'asc' });
	let orderBy = $state<SortField>(initialSort.field as SortField);
	let sortDirection = $state<SortDirection>(initialSort.direction);

	function getFromDateTimeUTC(): string {
		const [hour, minute] = (fromTime || '00:00').split(':').map(Number);
		const dt = calendarDateTimeToLuxon(
			{ year: fromDate.year, month: fromDate.month, day: fromDate.day, hour, minute },
			timezone
		);
		return toUTCISO(dt);
	}

	function getToDateTimeUTC(): string {
		const [hour, minute] = (toTime || '23:59').split(':').map(Number);
		const dt = calendarDateTimeToLuxon(
			{ year: toDate.year, month: toDate.month, day: toDate.day, hour, minute },
			timezone
		).endOf('minute');
		return toUTCISO(dt);
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
		loadData(false);
	}

	async function loadData(pushToHistory = true) {
		loading = true;
		error = '';

		if (selectedPreset) {
			const range = getTimeRangeFromPreset(selectedPreset, timezone);
			fromDate = dateToCalendarDate(range.from, timezone);
			toDate = dateToCalendarDate(range.to, timezone);
			fromTime = dateToTimeString(range.from, timezone);
			toTime = dateToTimeString(range.to, timezone);
		}

		updateTimeRangeUrl(pushToHistory);

		try {
			const response = await api.post(
				'/synthetics/overview',
				{ fromDate: getFromDateTimeUTC(), toDate: getToDateTimeUTC() },
				{ projectId: projectsState.currentProjectId ?? undefined }
			);
			checks = response.checks || [];
			monitorsState.refreshDownCount();
		} catch (e: any) {
			console.error(e);
			error = e.message || 'Failed to load data';
		} finally {
			loading = false;
		}
	}

	function handleSort(field: SortField) {
		const newSort = handleSortClick(field, orderBy, sortDirection);
		orderBy = newSort.field as SortField;
		sortDirection = newSort.direction;
		setSortState(SORT_STORAGE_KEY, newSort);
	}

	function handleSearch() {
		updateTimeRangeUrl(true);
	}

	function uptimePct(check: CheckOverview): number | null {
		const agg = check.aggregates;
		if (!agg || agg.total === 0) return null;
		const measured = agg.total - agg.missed;
		if (measured === 0) return null;
		return (agg.up / measured) * 100;
	}

	const statusRank: Record<string, number> = { down: 0, unknown: 1, up: 2 };

	const visibleChecks = $derived.by(() => {
		const query = searchQuery.trim().toLowerCase();
		let filtered = typeFilter ? checks.filter((c) => c.checkType === typeFilter) : checks;
		if (query) {
			filtered = filtered.filter(
				(c) => c.name.toLowerCase().includes(query) || c.checkType.includes(query)
			);
		}
		const direction = sortDirection === 'asc' ? 1 : -1;
		return [...filtered].sort((a, b) => {
			switch (orderBy) {
				case 'check_type':
					return direction * a.checkType.localeCompare(b.checkType);
				case 'status': {
					const rankA = a.enabled ? statusRank[a.currentStatus] : 3;
					const rankB = b.enabled ? statusRank[b.currentStatus] : 3;
					return direction * (rankA - rankB);
				}
				case 'uptime':
					return direction * ((uptimePct(a) ?? -1) - (uptimePct(b) ?? -1));
				case 'latency':
					return (
						direction * ((a.aggregates?.avgLatencyMs ?? -1) - (b.aggregates?.avgLatencyMs ?? -1))
					);
				case 'last_run': {
					const timeA = a.lastRunAt ? new Date(a.lastRunAt).getTime() : 0;
					const timeB = b.lastRunAt ? new Date(b.lastRunAt).getTime() : 0;
					return direction * (timeA - timeB);
				}
				default:
					return direction * a.name.localeCompare(b.name);
			}
		});
	});

	function formatRelative(iso: string | null): string {
		if (!iso) return 'never';
		const diffMs = Date.now() - new Date(iso).getTime();
		const diffSec = Math.floor(diffMs / 1000);
		if (diffSec < 60) return `${diffSec}s ago`;
		const diffMin = Math.floor(diffSec / 60);
		if (diffMin < 60) return `${diffMin}m ago`;
		const diffHours = Math.floor(diffMin / 60);
		if (diffHours < 24) return `${diffHours}h ago`;
		return `${Math.floor(diffHours / 24)}d ago`;
	}

	function uptimeClass(uptime: number): string {
		if (uptime >= 99.9) return 'text-green-600 dark:text-green-400';
		if (uptime >= 99) return '';
		return 'text-red-600 dark:text-red-400';
	}

	onMount(() => {
		window.addEventListener('popstate', handlePopState);
		if (activeTab === 'monitors') {
			loadData(false);
		}
	});

	onDestroy(() => {
		if (typeof window !== 'undefined') {
			window.removeEventListener('popstate', handlePopState);
		}
	});
</script>

<div class="space-y-4">
	<PageHeader title="Monitors" />

	<TabsRow
		tabs={TABS}
		{activeTab}
		onTabChange={setTab}
		actionLabel={newAction?.label}
		onAction={newAction?.onclick}
	/>

	<InfoCallout>{tabDescriptions[activeTab]}</InfoCallout>

	{#if activeTab === 'monitors'}
		<div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
			<SearchBar
				placeholder="Search monitors..."
				bind:value={searchQuery}
				bind:typeValue={typeFilter}
				{typeOptions}
				onSearch={handleSearch}
				disabled={loading}
			/>
			<div class="w-full sm:w-auto">
				<TimeRangePicker
					bind:fromDate
					bind:toDate
					bind:fromTime
					bind:toTime
					bind:preset={selectedPreset}
					onApply={handleTimeRangeChange}
				/>
			</div>
		</div>

		{#if loading}
			<div class="flex h-48 items-center justify-center">
				<LoadingCircle size="xlg" />
			</div>
		{:else if error}
			<div class="rounded-md border py-16 text-center text-red-500">{error}</div>
		{:else if checks.length === 0}
			{#if projectsState.canWriteCurrentProject}
				<EmptyState
					message="No monitors yet. Create your first Monitor to start tracking uptime."
					actionLabel="Create your first Monitor"
					onAction={() => (dialogOpen = true)}
				/>
			{:else}
				<EmptyState message="No monitors yet." />
			{/if}
		{:else}
			<div class="overflow-hidden rounded-md border">
				<Table.Root>
					{#if visibleChecks.length === 0}
						<Table.Body>
							<TableEmptyState colspan={6} message="No monitors match your search." />
						</Table.Body>
					{:else}
						<Table.Header>
							<Table.Row>
								<TracewayTableHeader
									label="Name"
									sortField="name"
									currentSortField={orderBy}
									{sortDirection}
									onSort={(field) => handleSort(field as SortField)}
								/>
								<TracewayTableHeader
									label="Type"
									sortField="check_type"
									currentSortField={orderBy}
									{sortDirection}
									onSort={(field) => handleSort(field as SortField)}
									class="w-[100px]"
								/>
								<TracewayTableHeader
									label="Status"
									tooltip="Current state from the most recent probes"
									sortField="status"
									currentSortField={orderBy}
									{sortDirection}
									onSort={(field) => handleSort(field as SortField)}
									class="w-[110px]"
								/>
								<TracewayTableHeader
									label="Uptime"
									tooltip="Share of successful probes in the selected time range (missed probes excluded)"
									sortField="uptime"
									currentSortField={orderBy}
									{sortDirection}
									onSort={(field) => handleSort(field as SortField)}
									class="w-[160px]"
								/>
								<TracewayTableHeader
									label="Avg latency"
									tooltip="Average probe latency in the selected time range"
									sortField="latency"
									currentSortField={orderBy}
									{sortDirection}
									onSort={(field) => handleSort(field as SortField)}
									class="w-[120px]"
								/>
								<TracewayTableHeader
									label="Last run"
									sortField="last_run"
									currentSortField={orderBy}
									{sortDirection}
									onSort={(field) => handleSort(field as SortField)}
									align="right"
									class="w-[110px]"
								/>
							</Table.Row>
						</Table.Header>
						<Table.Body>
							{#each visibleChecks as check}
								{@const uptime = uptimePct(check)}
								<Table.Row
									class="cursor-pointer hover:bg-muted/50"
									onclick={createRowClickHandler(
										resolve(`/monitors/${check.id}`),
										'preset',
										'from',
										'to'
									)}
								>
									<Table.Cell class="max-w-[40%] font-medium break-all whitespace-normal">
										{check.name}
										<div class="mt-0.5 font-mono text-xs font-normal text-muted-foreground">
											{#if check.checkType === 'http'}
												{check.config?.method || 'GET'}
												{check.config?.url}
											{:else if check.checkType === 'tcp'}
												{check.config?.host}:{check.config?.port}
											{:else}
												Playwright script
											{/if}
										</div>
									</Table.Cell>
									<Table.Cell>
										<MonitorTypeBadge type={check.checkType} />
									</Table.Cell>
									<Table.Cell>
										<CheckStatusBadge status={check.currentStatus} enabled={check.enabled} />
									</Table.Cell>
									<Table.Cell>
										{#if uptime === null}
											<span class="text-muted-foreground">—</span>
										{:else}
											<div class="flex items-center gap-2">
												<span
													class="w-16 shrink-0 font-mono text-sm tabular-nums {uptimeClass(uptime)}"
												>
													{uptime.toFixed(uptime === 100 ? 0 : 2)}%
												</span>
												<div class="h-1.5 w-14 shrink-0 overflow-hidden rounded-full bg-muted">
													<div
														class="h-full rounded-full {uptime >= 99.9
															? 'bg-green-500'
															: uptime >= 99
																? 'bg-amber-500'
																: 'bg-red-500'}"
														style="width: {Math.max(uptime, 4)}%"
													></div>
												</div>
											</div>
										{/if}
									</Table.Cell>
									<Table.Cell class="font-mono text-sm text-muted-foreground tabular-nums">
										{check.aggregates && check.aggregates.total - check.aggregates.missed > 0
											? formatDurationMs(check.aggregates.avgLatencyMs)
											: '—'}
									</Table.Cell>
									<Table.Cell class="text-right text-sm text-muted-foreground">
										{formatRelative(check.lastRunAt)}
									</Table.Cell>
								</Table.Row>
							{/each}
						</Table.Body>
					{/if}
				</Table.Root>
			</div>
		{/if}
	{:else if activeTab === 'status-pages'}
		<StatusPagesTab bind:this={statusPagesTab} />
	{/if}
</div>

<MonitorSheet bind:open={dialogOpen} check={null} onSaved={() => loadData(false)} />
