<script lang="ts">
	import { onMount } from 'svelte';
	import { browser } from '$app/environment';
	import { Button } from '$lib/components/ui/button';
	import { RefreshCw, Code } from 'lucide-svelte';
	import ServerFilter from '$lib/components/dashboard/server-filter.svelte';
	import MetricsTabContent from '$lib/components/dashboard/metrics-tab-content.svelte';
	import * as Tabs from '$lib/components/ui/tabs';
	import type {
		DashboardMetric,
		ServerMetricTrend,
		MetricTrendPoint,
		MetricsTab,
		ApplicationMetricsData,
		StatsMetricsData,
		ServerMetricsData
	} from '$lib/types/dashboard';
	import { api } from '$lib/api';
	import { projectsState } from '$lib/state/projects.svelte';
	import { getTimezone } from '$lib/state/timezone.svelte';
	import { toUTCISO, calendarDateTimeToLuxon, formatDateTime } from '$lib/utils/formatters';
	import { TimeRangePicker } from '$lib/components/ui/time-range-picker';
	import { CalendarDate } from '@internationalized/date';
	import { getServerColorMap } from '$lib/utils/server-colors';
	import {
		presetMinutes,
		getTimeRangeFromPreset,
		dateToCalendarDate,
		dateToTimeString,
		parseTimeRangeFromUrl,
		getResolvedTimeRange,
		updateUrl
	} from '$lib/utils/url-params';

	const timezone = $derived(getTimezone());

	// Tab state
	let activeTab = $state<MetricsTab>('application');

	// Per-tab data (null = not loaded yet)
	let applicationData = $state<ApplicationMetricsData | null>(null);
	let statsData = $state<StatsMetricsData | null>(null);
	let serverData = $state<ServerMetricsData | null>(null);

	// Per-tab loading states
	let loadingApplication = $state(false);
	let loadingStats = $state(false);
	let loadingServer = $state(false);

	// Per-tab error states
	let errorApplication = $state('');
	let errorStats = $state('');
	let errorServer = $state('');

	// Server filtering state (client-side only)
	let selectedServers = $state<string[]>([]);

	// Merge available servers from all endpoints
	const availableServers = $derived(() => {
		const servers = new Set<string>();
		applicationData?.availableServers?.forEach(s => servers.add(s));
		serverData?.availableServers?.forEach(s => servers.add(s));
		return Array.from(servers).sort();
	});

	// Compute server color map for consistent colors across all charts
	const serverColorMap = $derived(getServerColorMap(availableServers()));

	// Parse extended URL params (includes servers and tab)
	function parseMetricsUrlParams(): {
		preset: string | null;
		from: Date | null;
		to: Date | null;
		servers: string[];
		tab: MetricsTab;
	} {
		if (!browser) return { preset: '24h', from: null, to: null, servers: [], tab: 'application' };

		const params = new URLSearchParams(window.location.search);
		const serversParam = params.get('servers');
		const tabParam = params.get('tab') as MetricsTab | null;

		const servers = serversParam ? serversParam.split(',').filter((s) => s.length > 0) : [];
		const tab: MetricsTab = tabParam && ['application', 'stats', 'server'].includes(tabParam)
			? tabParam
			: 'application';

		const timeParams = parseTimeRangeFromUrl(timezone);
		return { ...timeParams, servers, tab };
	}

	// Initialize from URL or defaults
	const initialUrlParams = parseMetricsUrlParams();
	const initialRange = getResolvedTimeRange(initialUrlParams, timezone);

	let selectedPreset = $state<string | null>(initialUrlParams.preset);
	let fromDate = $state<CalendarDate>(dateToCalendarDate(initialRange.from, timezone));
	let toDate = $state<CalendarDate>(dateToCalendarDate(initialRange.to, timezone));
	let fromTime = $state(dateToTimeString(initialRange.from, timezone));
	let toTime = $state(dateToTimeString(initialRange.to, timezone));
	let sharedTimeDomain = $state<[Date, Date] | null>(null);

	// Initialize tab from URL
	activeTab = initialUrlParams.tab;

	// Update URL with current state
	function updateMetricsUrl(pushToHistory = true) {
		if (!browser) return;

		const params: Record<string, string | null | undefined> = {
			tab: activeTab
		};

		if (selectedPreset) {
			params.preset = selectedPreset;
		} else {
			params.from = getFromDateTimeUTC();
			params.to = getToDateTimeUTC();
		}

		if (selectedServers.length > 0 && selectedServers.length < availableServers().length) {
			params.servers = selectedServers.join(',');
		}

		updateUrl(params, { pushToHistory });
	}

	// Handle browser back/forward navigation
	function handlePopState() {
		const urlParams = parseMetricsUrlParams();
		const range = getResolvedTimeRange(urlParams, timezone);

		selectedPreset = urlParams.preset;
		fromDate = dateToCalendarDate(range.from, timezone);
		fromTime = dateToTimeString(range.from, timezone);
		toDate = dateToCalendarDate(range.to, timezone);
		toTime = dateToTimeString(range.to, timezone);
		selectedServers = urlParams.servers;
		activeTab = urlParams.tab;

		// Clear cached data and reload active tab
		clearAllData();
		loadActiveTab(false);
	}

	function getFromDateTimeUTC(): string {
		const [hour, minute] = (fromTime || '00:00').split(':').map(Number);
		const dt = calendarDateTimeToLuxon({ year: fromDate.year, month: fromDate.month, day: fromDate.day, hour, minute }, timezone);
		return toUTCISO(dt);
	}

	function getToDateTimeUTC(): string {
		const [hour, minute] = (toTime || '23:59').split(':').map(Number);
		const dt = calendarDateTimeToLuxon({ year: toDate.year, month: toDate.month, day: toDate.day, hour, minute }, timezone);
		return toUTCISO(dt);
	}

	// Clear all cached data
	function clearAllData() {
		applicationData = null;
		statsData = null;
		serverData = null;
	}

	// Transform API response timestamps to Date objects
	function transformMetrics(metrics: any[]): DashboardMetric[] {
		if (!metrics || !Array.isArray(metrics)) return [];
		return metrics.map((m: any) => ({
			id: m.id,
			name: m.name,
			value: m.value,
			unit: m.unit,
			trend: (m.trend || []).map((t: any) => ({
				timestamp: new Date(t.timestamp),
				value: t.value
			})),
			status: m.status,
			servers: m.servers?.map(
				(s: any): ServerMetricTrend => ({
					serverName: s.serverName,
					value: s.value,
					trend: (s.trend || []).map(
						(t: any): MetricTrendPoint => ({
							timestamp: new Date(t.timestamp),
							value: t.value
						})
					)
				})
			)
		}));
	}

	// Load Application metrics
	async function loadApplicationMetrics() {
		loadingApplication = true;
		errorApplication = '';

		try {
			const queryParams = `fromDate=${getFromDateTimeUTC()}&toDate=${getToDateTimeUTC()}`;
			const response = await api.get(`/metrics/application?${queryParams}`, {
				projectId: projectsState.currentProjectId ?? undefined
			});

			applicationData = {
				metrics: transformMetrics(response.metrics),
				availableServers: response.availableServers || [],
				lastUpdated: response.lastUpdated ? new Date(response.lastUpdated) : new Date()
			};
		} catch (e: any) {
			errorApplication = e.message || 'Failed to load application metrics';
			console.error(e);
		} finally {
			loadingApplication = false;
		}
	}

	// Load Stats metrics
	async function loadStatsMetrics() {
		loadingStats = true;
		errorStats = '';

		try {
			const queryParams = `fromDate=${getFromDateTimeUTC()}&toDate=${getToDateTimeUTC()}`;
			const response = await api.get(`/metrics/stats?${queryParams}`, {
				projectId: projectsState.currentProjectId ?? undefined
			});

			statsData = {
				metrics: transformMetrics(response.metrics),
				lastUpdated: response.lastUpdated ? new Date(response.lastUpdated) : new Date()
			};
		} catch (e: any) {
			errorStats = e.message || 'Failed to load stats metrics';
			console.error(e);
		} finally {
			loadingStats = false;
		}
	}

	// Load Server metrics
	async function loadServerMetrics() {
		loadingServer = true;
		errorServer = '';

		try {
			const queryParams = `fromDate=${getFromDateTimeUTC()}&toDate=${getToDateTimeUTC()}`;
			const response = await api.get(`/metrics/server?${queryParams}`, {
				projectId: projectsState.currentProjectId ?? undefined
			});

			serverData = {
				metrics: transformMetrics(response.metrics),
				availableServers: response.availableServers || [],
				lastUpdated: response.lastUpdated ? new Date(response.lastUpdated) : new Date()
			};
		} catch (e: any) {
			errorServer = e.message || 'Failed to load server metrics';
			console.error(e);
		} finally {
			loadingServer = false;
		}
	}

	// Load only the active tab's data (lazy loading)
	function loadActiveTab(pushToHistory = true) {
		if (pushToHistory) updateMetricsUrl(true);

		// Update shared time domain
		sharedTimeDomain = [new Date(getFromDateTimeUTC()), new Date(getToDateTimeUTC())];

		switch (activeTab) {
			case 'application':
				if (!applicationData) loadApplicationMetrics();
				break;
			case 'stats':
				if (!statsData) loadStatsMetrics();
				break;
			case 'server':
				if (!serverData) loadServerMetrics();
				break;
		}
	}

	// Force reload (for refresh button & time range change)
	function reloadActiveTab() {
		updateMetricsUrl(true);
		sharedTimeDomain = [new Date(getFromDateTimeUTC()), new Date(getToDateTimeUTC())];

		// Clear all cached data when time range changes
		clearAllData();

		// Load the active tab
		switch (activeTab) {
			case 'application': loadApplicationMetrics(); break;
			case 'stats': loadStatsMetrics(); break;
			case 'server': loadServerMetrics(); break;
		}
	}

	// Handle time range change
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
		reloadActiveTab();
	}

	// Handle tab change - lazy load if data not loaded
	function handleTabChange(tab: string) {
		activeTab = tab as MetricsTab;
		loadActiveTab(false);
		updateMetricsUrl(false);
	}

	// Client-side server filtering - NO API call needed
	function handleServerSelectionChange(servers: string[]) {
		selectedServers = servers;
		updateMetricsUrl(false);
	}

	// Handle drag-to-zoom selection from chart overlay
	function handleChartRangeSelect(from: Date, to: Date) {
		selectedPreset = null;
		fromDate = new CalendarDate(from.getFullYear(), from.getMonth() + 1, from.getDate());
		fromTime = `${String(from.getHours()).padStart(2, '0')}:${String(from.getMinutes()).padStart(2, '0')}`;
		toDate = new CalendarDate(to.getFullYear(), to.getMonth() + 1, to.getDate());
		toTime = `${String(to.getHours()).padStart(2, '0')}:${String(to.getMinutes()).padStart(2, '0')}`;
		reloadActiveTab();
	}

	// Get last updated time for current tab
	const lastUpdated = $derived(() => {
		switch (activeTab) {
			case 'application': return applicationData?.lastUpdated;
			case 'stats': return statsData?.lastUpdated;
			case 'server': return serverData?.lastUpdated;
			default: return undefined;
		}
	});

	const lastUpdatedFormatted = $derived(
		lastUpdated()
			? formatDateTime(lastUpdated()!, { timezone, format: 'time' })
			: ''
	);

	// Check if current tab is loading
	const isCurrentTabLoading = $derived(() => {
		switch (activeTab) {
			case 'application': return loadingApplication;
			case 'stats': return loadingStats;
			case 'server': return loadingServer;
			default: return false;
		}
	});

	onMount(() => {
		// Only load the active tab on mount (lazy loading)
		loadActiveTab(false);

		window.addEventListener('popstate', handlePopState);
		return () => window.removeEventListener('popstate', handlePopState);
	});
</script>

<div class="space-y-4">
	<!-- Header -->
	<div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
		<h2 class="text-2xl font-bold tracking-tight">Metrics</h2>
		<div class="flex items-center gap-2">
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

	<!-- Tabs -->
	<Tabs.Root bind:value={activeTab} onValueChange={handleTabChange}>
		<div class="flex flex-wrap items-center justify-between gap-2">
			<div class="flex flex-wrap items-center gap-2">
				<Tabs.List>
					<Tabs.Trigger value="application">Application</Tabs.Trigger>
					<Tabs.Trigger value="stats">Stats</Tabs.Trigger>
					<Tabs.Trigger value="server">CPU / Mem</Tabs.Trigger>
					<Tabs.Trigger value="custom">Custom</Tabs.Trigger>
				</Tabs.List>

				{#if availableServers().length > 1}
					<ServerFilter
						availableServers={availableServers()}
						bind:selectedServers
						onSelectionChange={handleServerSelectionChange}
						{serverColorMap}
					/>
				{/if}
			</div>

			{#if lastUpdated()}
				<div class="flex items-center gap-1">
					<span class="text-sm text-muted-foreground whitespace-nowrap">
						Updated: {lastUpdatedFormatted}
					</span>

					<Button variant="ghost" size="sm" onclick={() => reloadActiveTab()} disabled={isCurrentTabLoading()}>
						<RefreshCw class="h-4 w-4 {isCurrentTabLoading() ? 'animate-spin' : ''}" />
					</Button>
				</div>
			{/if}
		</div>

		<!-- Application Tab -->
		<Tabs.Content value="application">
			<MetricsTabContent
				metrics={applicationData?.metrics ?? null}
				loading={loadingApplication}
				error={errorApplication}
				errorTitle="Failed to Load Application Metrics"
				onRetry={() => loadApplicationMetrics()}
				timeDomain={sharedTimeDomain}
				onRangeSelect={handleChartRangeSelect}
				{serverColorMap}
				{selectedServers}
				availableServers={availableServers()}
			/>
		</Tabs.Content>

		<!-- Stats Tab -->
		<Tabs.Content value="stats">
			<MetricsTabContent
				metrics={statsData?.metrics ?? null}
				loading={loadingStats}
				error={errorStats}
				errorTitle="Failed to Load Stats Metrics"
				onRetry={() => loadStatsMetrics()}
				timeDomain={sharedTimeDomain}
				onRangeSelect={handleChartRangeSelect}
				{serverColorMap}
				{selectedServers}
				availableServers={availableServers()}
			/>
		</Tabs.Content>

		<!-- Server Tab -->
		<Tabs.Content value="server">
			<MetricsTabContent
				metrics={serverData?.metrics ?? null}
				loading={loadingServer}
				error={errorServer}
				errorTitle="Failed to Load Server Metrics"
				onRetry={() => loadServerMetrics()}
				timeDomain={sharedTimeDomain}
				onRangeSelect={handleChartRangeSelect}
				{serverColorMap}
				{selectedServers}
				availableServers={availableServers()}
			/>
		</Tabs.Content>

		<!-- Custom Tab -->
		<Tabs.Content value="custom">
			<div class="flex flex-col items-center justify-center py-8 text-center">
				<div class="bg-muted mb-4 rounded-full p-3">
					<Code class="text-muted-foreground h-6 w-6" />
				</div>
				<h3 class="mb-2 text-lg font-semibold">Custom Metrics not supported</h3>
				<p class="text-muted-foreground mb-4 max-w-md text-sm">
					Custom Metrics are not currently supported, but we're working on it and hope to have them supported soon.
				</p>
			</div>
		</Tabs.Content>
	</Tabs.Root>
</div>
