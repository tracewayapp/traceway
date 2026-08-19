<script lang="ts">
	import { getErrorMessage, getErrorStatus } from '$lib/utils/errors';
	import { onMount } from 'svelte';
	import { SvelteURLSearchParams } from 'svelte/reactivity';
	import { api } from '$lib/api';
	import { formatDurationMs, formatDateTime } from '$lib/utils/formatters';
	import * as Table from '$lib/components/ui/table';
	import * as Card from '$lib/components/ui/card';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Button } from '$lib/components/ui/button';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { ErrorDisplay } from '$lib/components/ui/error-display';
	import { TableEmptyState } from '$lib/components/ui/table-empty-state';
	import { PaginationFooter } from '$lib/components/ui/pagination-footer';
	import CheckStatusBadge from '$lib/components/synthetics/check-status-badge.svelte';
	import MonitorTypeBadge from '$lib/components/synthetics/monitor-type-badge.svelte';
	import MonitorSheet from '../monitor-sheet.svelte';
	import NewPostMortemDialog from '../new-post-mortem-dialog.svelte';
	import D3LineChart from '$lib/components/dashboard/d3-line-chart.svelte';
	import PageHeader from '$lib/components/traceway/page-header.svelte';
	import StatRow from '$lib/components/traceway/stat-row.svelte';
	import StatTile from '$lib/components/traceway/stat-tile.svelte';
	import StatusPill from '$lib/components/traceway/status-pill.svelte';
	import ChartCard from '$lib/components/traceway/chart-card.svelte';
	import ToolbarSelect from '$lib/components/traceway/toolbar-select.svelte';
	import TableContainer from '$lib/components/traceway/table-container.svelte';
	import ConfirmDeleteDialog from '$lib/components/traceway/confirm-delete-dialog.svelte';
	import { uptimeClass } from '$lib/utils/uptime';
	import { browser } from '$app/environment';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { projectsState } from '$lib/state/projects.svelte';
	import {
		monitorsState,
		incidentDisplayTitle,
		type SyntheticCheck,
		type CheckIncident,
		type CheckResultEntry,
		type CheckSeriesPoint
	} from '$lib/state/monitors.svelte';
	import { createSmartBackHandler } from '$lib/utils/back-navigation';
	import { authState } from '$lib/state/auth.svelte';
	import {
		Activity,
		ChartLine,
		FileText,
		History,
		Image as ImageIcon,
		Pause,
		Pencil,
		Play,
		ScrollText,
		Trash2,
		TriangleAlert
	} from '@lucide/svelte';
	import { toast } from 'svelte-sonner';

	let { data } = $props();

	const checkId = data.checkId;

	const canWritePostMortems = $derived.by(() => {
		const orgId = projectsState.currentProject?.organizationId;
		if (!orgId) return false;
		return authState.canWriteOrganization(orgId);
	});

	let check = $state<SyntheticCheck | null>(null);
	let incidents = $state<CheckIncident[]>([]);
	let loading = $state(true);
	let error = $state('');
	let notFound = $state(false);

	let series = $state<CheckSeriesPoint[]>([]);
	let seriesLoading = $state(true);

	let results = $state<CheckResultEntry[]>([]);
	let resultsLoading = $state(true);
	let page = $state(1);
	let pageSize = $state(20);
	let total = $state(0);
	let totalPages = $state(0);

	// Result filter for the runs table; the date range is the page's fixed
	// 30-day window.
	let resultFilter = $state('all');
	const resultFilterOptions = [
		{ value: 'all', label: 'All results' },
		{ value: 'up', label: 'Up' },
		{ value: 'down', label: 'Down' },
		{ value: 'missed', label: 'Missed' }
	];
	function onResultFilterChange() {
		page = 1;
		loadResults();
	}

	let editOpen = $state(false);
	let deleteOpen = $state(false);
	let newPostMortemOpen = $state(false);
	let newPostMortemIncident = $state<CheckIncident | null>(null);
	let deleting = $state(false);
	let runningNow = $state(false);
	let togglingPause = $state(false);

	// The whole page shows a fixed window of the most recent runs — no time
	// filter. The window is refreshed on every load so charts and the run
	// history stay anchored to "now".
	const WINDOW_DAYS = 30;
	const initialTo = new Date();
	let rangeTo = $state(initialTo);
	let rangeFrom = $state(new Date(initialTo.getTime() - WINDOW_DAYS * 24 * 60 * 60 * 1000));

	// Snap to a step that keeps the availability strip readable for every
	// range (a 3M range at 240m would be 540 cells and overflow the row).
	// The bucket target shrinks with the viewport so the strip never forces
	// the page into horizontal scrolling on phones. The small tolerance keeps
	// a range that is one endOf-minute epsilon over a boundary on the finer
	// step.
	const intervalSteps = [1, 5, 15, 30, 60, 120, 240, 480, 720, 1440];

	function bucketTarget(): number {
		if (!browser) return 96;
		// ~7px per cell (4px bar + 3px gap); reserve room for the sidebar and
		// paddings on desktop, just the paddings on phones.
		const chrome = window.innerWidth >= 768 ? 380 : 90;
		return Math.max(24, Math.min(96, Math.floor((window.innerWidth - chrome) / 7)));
	}

	function calculateInterval(from: Date, to: Date): number {
		const raw = (to.getTime() - from.getTime()) / 60000 / bucketTarget();
		for (const step of intervalSteps) {
			if (step >= raw - 0.05) return step;
		}
		return 1440;
	}

	async function loadCheck() {
		try {
			const response = await api.get(`/synthetics/checks/${checkId}`, {
				projectId: projectsState.currentProjectId ?? undefined
			});
			check = response.check;
			incidents = response.incidents || [];
		} catch (e) {
			if (getErrorStatus(e) === 404) {
				notFound = true;
			} else {
				error = getErrorMessage(e) || 'Failed to load the monitor';
			}
		}
	}

	async function loadSeries() {
		seriesLoading = true;
		try {
			const response = await api.post(
				`/synthetics/checks/${checkId}/series`,
				{
					fromDate: rangeFrom.toISOString(),
					toDate: rangeTo.toISOString(),
					intervalMinutes: calculateInterval(rangeFrom, rangeTo)
				},
				{ projectId: projectsState.currentProjectId ?? undefined }
			);
			series = response.series || [];
		} catch (e) {
			console.error('Failed to load check series:', e);
			series = [];
		} finally {
			seriesLoading = false;
		}
	}

	async function loadResults() {
		resultsLoading = true;
		try {
			const response = await api.post(
				`/synthetics/checks/${checkId}/results`,
				{
					fromDate: rangeFrom.toISOString(),
					toDate: rangeTo.toISOString(),
					status: resultFilter === 'all' ? '' : resultFilter,
					pagination: { page, pageSize }
				},
				{ projectId: projectsState.currentProjectId ?? undefined }
			);
			results = response.data || [];
			total = response.pagination.total;
			totalPages = response.pagination.totalPages;
		} catch (e) {
			console.error('Failed to load check results:', e);
			results = [];
		} finally {
			resultsLoading = false;
		}
	}

	async function loadAll() {
		loading = true;
		error = '';
		notFound = false;

		rangeTo = new Date();
		rangeFrom = new Date(rangeTo.getTime() - WINDOW_DAYS * 24 * 60 * 60 * 1000);

		await loadCheck();
		loading = false;
		if (!notFound && !error) {
			loadSeries();
			loadResults();
		}
	}

	async function runNow() {
		runningNow = true;
		try {
			await api.post(
				`/synthetics/checks/${checkId}/run`,
				{},
				{ projectId: projectsState.currentProjectId ?? undefined }
			);
			toast.success('Run queued');
			setTimeout(() => {
				loadCheck();
				loadResults();
			}, 3000);
		} catch (e) {
			if (getErrorStatus(e) !== 403) {
				toast.error(getErrorMessage(e) || 'Failed to queue the run');
			}
		} finally {
			runningNow = false;
		}
	}

	async function togglePause() {
		if (!check) return;
		togglingPause = true;
		try {
			const response = await api.put(
				`/synthetics/checks/${checkId}`,
				{
					name: check.name,
					checkType: check.checkType,
					enabled: !check.enabled,
					intervalSeconds: check.intervalSeconds,
					timeoutSeconds: check.timeoutSeconds,
					failureThreshold: check.failureThreshold,
					config: check.config
				},
				{ projectId: projectsState.currentProjectId ?? undefined }
			);
			check = response;
			toast.success('Successfully updated the Monitor', { position: 'top-center' });
		} catch (e) {
			if (getErrorStatus(e) !== 403) {
				toast.error(getErrorMessage(e) || 'Failed to update the monitor');
			}
		} finally {
			togglingPause = false;
		}
	}

	async function deleteCheck() {
		deleting = true;
		try {
			await api.delete(`/synthetics/checks/${checkId}`, {
				projectId: projectsState.currentProjectId ?? undefined
			});
			toast.success('Successfully deleted the Monitor');
			monitorsState.refreshDownCount();
			goto(resolve('/monitors'));
		} catch (e) {
			if (getErrorStatus(e) !== 403) {
				toast.error(getErrorMessage(e) || 'Failed to delete the monitor');
			}
			deleting = false;
			deleteOpen = false;
		}
	}

	const rangeAggregates = $derived.by(() => {
		let up = 0;
		let measured = 0;
		let latencySum = 0;
		for (const point of series) {
			up += point.up;
			measured += point.total - point.missed;
			latencySum += point.avgLatencyMs * (point.total - point.missed);
		}
		return {
			uptime: measured > 0 ? (up / measured) * 100 : null,
			avgLatency: measured > 0 ? latencySum / measured : null
		};
	});

	const latencySeries = $derived.by(() => [
		{
			key: 'Avg latency',
			color: 'var(--chart-1)',
			data: series
				.filter((p) => p.total - p.missed > 0)
				.map((p) => ({ timestamp: new Date(p.bucket), value: p.avgLatencyMs }))
		}
	]);

	// The backend only returns buckets that have rows; rebuild the full
	// epoch-floored grid over the selected range so sparse data renders as
	// explicit no-data cells instead of a few bars stretched across the strip.
	const paddedSeries = $derived.by(() => {
		if (series.length === 0) return [] as CheckSeriesPoint[];
		const fromMs = rangeFrom.getTime();
		const toMs = rangeTo.getTime();
		const stepMs = calculateInterval(rangeFrom, rangeTo) * 60000;
		const byBucket = new Map(series.map((p) => [new Date(p.bucket).getTime(), p]));
		const cells: CheckSeriesPoint[] = [];
		for (
			let t = Math.floor(fromMs / stepMs) * stepMs;
			t <= toMs && cells.length < 400;
			t += stepMs
		) {
			cells.push(
				byBucket.get(t) ?? {
					bucket: new Date(t).toISOString(),
					total: 0,
					up: 0,
					missed: 0,
					avgLatencyMs: 0,
					maxLatencyMs: 0
				}
			);
		}
		return cells;
	});

	function bucketClass(point: CheckSeriesPoint): string {
		const measured = point.total - point.missed;
		if (measured === 0) return 'bg-muted';
		if (point.up === measured) return 'bg-green-500';
		if (point.up === 0) return 'bg-red-500';
		return 'bg-amber-500';
	}

	function bucketTitle(point: CheckSeriesPoint): string {
		const measured = point.total - point.missed;
		const when = formatDateTime(point.bucket, { format: 'short' });
		if (measured === 0) return `${when}: no data`;
		return `${when}: ${point.up}/${measured} up${point.missed > 0 ? `, ${point.missed} missed` : ''}`;
	}

	let screenshotOpen = $state(false);
	let screenshotUrl = $state('');
	let screenshotLoading = $state(false);

	async function viewScreenshot(key: string) {
		screenshotOpen = true;
		screenshotLoading = true;
		if (screenshotUrl) {
			URL.revokeObjectURL(screenshotUrl);
			screenshotUrl = '';
		}
		try {
			const params = new SvelteURLSearchParams({ key });
			if (projectsState.currentProjectId) {
				params.set('projectId', projectsState.currentProjectId);
			}
			const response = await fetch(`/api/synthetics/screenshot?${params}`, {
				headers: { Authorization: `Bearer ${authState.token}` }
			});
			if (!response.ok) throw new Error('Screenshot not found');
			screenshotUrl = URL.createObjectURL(await response.blob());
		} catch {
			screenshotOpen = false;
			toast.error('Failed to load the screenshot');
		} finally {
			screenshotLoading = false;
		}
	}

	let errorOpen = $state(false);
	let errorText = $state('');

	function viewError(message: string) {
		errorText = message;
		errorOpen = true;
	}

	// Browser runs have no HTTP status code; http/tcp runs have no artifacts.
	const isBrowserCheck = $derived(check?.checkType === 'browser');
	const runHistoryColumns = $derived(isBrowserCheck ? 6 : 5);

	let outputOpen = $state(false);
	let outputText = $state('');
	let outputLoading = $state(false);

	async function viewOutput(key: string) {
		outputOpen = true;
		outputLoading = true;
		outputText = '';
		try {
			const params = new SvelteURLSearchParams({ key });
			if (projectsState.currentProjectId) {
				params.set('projectId', projectsState.currentProjectId);
			}
			const response = await fetch(`/api/synthetics/output?${params}`, {
				headers: { Authorization: `Bearer ${authState.token}` }
			});
			if (!response.ok) throw new Error('Output not found');
			outputText = await response.text();
		} catch {
			outputOpen = false;
			toast.error('Failed to load the run output');
		} finally {
			outputLoading = false;
		}
	}

	function handlePageChange(newPage: number) {
		if (newPage >= 1 && newPage <= totalPages) {
			page = newPage;
			loadResults();
		}
	}

	function handlePageSizeChange(newPageSize: number) {
		pageSize = newPageSize;
		page = 1;
		loadResults();
	}

	onMount(() => {
		loadAll();
	});
</script>

{#if loading}
	<div class="flex h-64 items-center justify-center">
		<LoadingCircle size="xlg" />
	</div>
{:else if notFound}
	<ErrorDisplay
		status={404}
		title="Monitor not found"
		description="This monitor does not exist or belongs to a different project."
		onBack={createSmartBackHandler({ fallbackPath: '/monitors' })}
		backLabel="Back to Monitors"
	/>
{:else if error}
	<ErrorDisplay
		status={400}
		title="Error"
		description={error}
		onRetry={() => loadAll()}
		onBack={createSmartBackHandler({ fallbackPath: '/monitors' })}
		backLabel="Back to Monitors"
	/>
{:else if check}
	<div class="space-y-4">
		<PageHeader title={check.name} onBack={createSmartBackHandler({ fallbackPath: '/monitors' })}>
			{#snippet meta()}
				{#if check}
					<CheckStatusBadge status={check.currentStatus} enabled={check.enabled} />
					<MonitorTypeBadge type={check.checkType} />
					<span class="min-w-0 font-mono text-sm break-all text-muted-foreground">
						{#if check.checkType === 'http'}
							{check.config?.method || 'GET'} {check.config?.url}
						{:else if check.checkType === 'tcp'}
							{check.config?.host}:{check.config?.port}
						{/if}
					</span>
					<span class="text-sm text-muted-foreground">Last {WINDOW_DAYS} days</span>
				{/if}
			{/snippet}
			{#snippet actions()}
				{#if check && projectsState.canWriteCurrentProject}
					<Button
						variant="outline"
						size="sm"
						onclick={runNow}
						disabled={runningNow || !check.enabled}
					>
						<Play class="mr-2 h-4 w-4" />
						{runningNow ? 'Queueing...' : 'Run now'}
					</Button>
					<Button variant="outline" size="sm" onclick={togglePause} disabled={togglingPause}>
						{#if check.enabled}
							<Pause class="mr-2 h-4 w-4" />
							Pause
						{:else}
							<Play class="mr-2 h-4 w-4" />
							Resume
						{/if}
					</Button>
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
				label="Uptime"
				value={rangeAggregates.uptime === null
					? '—'
					: `${rangeAggregates.uptime.toFixed(rangeAggregates.uptime === 100 ? 0 : 2)}%`}
				valueClass={rangeAggregates.uptime === null
					? 'text-muted-foreground'
					: uptimeClass(rangeAggregates.uptime)}
			/>
			<StatTile
				label="Avg latency"
				value={rangeAggregates.avgLatency === null
					? '—'
					: formatDurationMs(rangeAggregates.avgLatency)}
			/>
			<StatTile
				label="Interval"
				value={check.intervalSeconds >= 3600
					? `${check.intervalSeconds / 3600}h`
					: check.intervalSeconds >= 60
						? `${check.intervalSeconds / 60}m`
						: `${check.intervalSeconds}s`}
			/>
			<StatTile
				label="Incidents (recent)"
				value={incidents.length}
				valueClass={incidents.some((i) => !i.resolvedAt) ? 'text-red-600 dark:text-red-400' : ''}
			/>
		</StatRow>

		<ChartCard
			title="Availability"
			loading={seriesLoading}
			empty={series.length === 0}
			emptyMessage="No probe data in the last {WINDOW_DAYS} days"
			height={64}
		>
			{#snippet icon()}
				<Activity class="h-3.5 w-3.5 text-muted-foreground" />
			{/snippet}
			<div class="flex h-9 items-stretch gap-[3px]">
				{#each paddedSeries as point, __index (__index)}
					<div
						class="min-w-0 flex-1 rounded-[3px] transition-transform hover:scale-y-110 {bucketClass(
							point
						)}"
						title={bucketTitle(point)}
					></div>
				{/each}
			</div>
			<div class="mt-1.5 flex justify-between text-xs text-muted-foreground">
				<span>{formatDateTime(paddedSeries[0].bucket, { format: 'short' })}</span>
				<span>Now</span>
			</div>
		</ChartCard>

		<ChartCard
			title="Latency"
			loading={seriesLoading}
			empty={latencySeries[0].data.length === 0}
			emptyMessage="No latency data in the last {WINDOW_DAYS} days"
		>
			{#snippet icon()}
				<ChartLine class="h-3.5 w-3.5 text-muted-foreground" />
			{/snippet}
			<D3LineChart series={latencySeries} formatValue={formatDurationMs} unit="ms" />
		</ChartCard>

		<Card.Root class="gap-0 pt-2 pb-0">
			<Card.Header
				class="flex flex-row flex-wrap items-center justify-between gap-3 space-y-0 border-b pl-3 [.border-b]:pb-2"
			>
				<Card.Title class="flex items-center gap-2 text-base font-medium">
					<History class="h-3.5 w-3.5 text-muted-foreground" />
					<span class="text-sm font-[400] text-muted-foreground">Run history</span>
				</Card.Title>
				<ToolbarSelect
					bind:value={resultFilter}
					options={resultFilterOptions}
					class="w-[130px]"
					onChange={onResultFilterChange}
				/>
			</Card.Header>
			<Card.Content class="overflow-hidden rounded-b-xl p-0">
				<TableContainer class="rounded-none border-0" minWidth="640px">
					<Table.Root>
						{#if resultsLoading}
							<Table.Body>
								<Table.Row>
									<Table.Cell colspan={runHistoryColumns} class="h-48">
										<div class="flex h-full items-center justify-center">
											<LoadingCircle size="xlg" />
										</div>
									</Table.Cell>
								</Table.Row>
							</Table.Body>
						{:else if results.length === 0}
							<Table.Body>
								<TableEmptyState
									colspan={runHistoryColumns}
									message="No runs in the last {WINDOW_DAYS} days"
								/>
							</Table.Body>
						{:else}
							<Table.Header>
								<Table.Row>
									<Table.Head>Time</Table.Head>
									<Table.Head class="w-[100px]">Result</Table.Head>
									<Table.Head class="w-[110px]">Latency</Table.Head>
									{#if !isBrowserCheck}
										<Table.Head class="w-[90px]">Status code</Table.Head>
									{/if}
									<Table.Head>Error</Table.Head>
									{#if isBrowserCheck}
										<Table.Head class="w-[44px]"><span class="sr-only">Screenshot</span></Table.Head
										>
										<Table.Head class="w-[44px]"><span class="sr-only">Logs</span></Table.Head>
									{/if}
								</Table.Row>
							</Table.Header>
							<Table.Body>
								{#each results as result, __index (__index)}
									<Table.Row>
										<Table.Cell class="text-sm whitespace-nowrap">
											{formatDateTime(result.recordedAt, { format: 'short' })}
											<span class="ml-1 text-xs text-muted-foreground">via {result.executedBy}</span
											>
										</Table.Cell>
										<Table.Cell>
											{#if result.status === 'up'}
												<StatusPill tone="success" dot label="Up" />
											{:else if result.status === 'down'}
												<StatusPill tone="danger" label="Down" />
											{:else}
												<StatusPill tone="neutral" label="Missed" />
											{/if}
										</Table.Cell>
										<Table.Cell class="font-mono text-sm text-muted-foreground tabular-nums">
											{result.status === 'missed' ? '—' : formatDurationMs(result.latencyMs)}
										</Table.Cell>
										{#if !isBrowserCheck}
											<Table.Cell class="font-mono text-sm text-muted-foreground tabular-nums">
												{result.statusCode > 0 ? result.statusCode : '—'}
											</Table.Cell>
										{/if}
										<Table.Cell class="max-w-[300px] min-w-[120px] text-xs text-muted-foreground">
											{#if result.errorMessage}
												<button
													type="button"
													class="line-clamp-2 w-full cursor-pointer text-left break-all"
													title="View the full error"
													onclick={() => viewError(result.errorMessage)}
												>
													{result.errorMessage}
												</button>
											{/if}
										</Table.Cell>
										{#if isBrowserCheck}
											<Table.Cell class="px-1 text-center">
												{#if result.screenshotKey}
													<Button
														variant="ghost"
														size="icon"
														class="h-7 w-7"
														title="View the screenshot"
														onclick={() => viewScreenshot(result.screenshotKey)}
													>
														<ImageIcon class="h-4 w-4" />
													</Button>
												{/if}
											</Table.Cell>
											<Table.Cell class="px-1 text-center">
												{#if result.outputKey}
													<Button
														variant="ghost"
														size="icon"
														class="h-7 w-7"
														title="View the logs"
														onclick={() => viewOutput(result.outputKey)}
													>
														<ScrollText class="h-4 w-4" />
													</Button>
												{/if}
											</Table.Cell>
										{/if}
									</Table.Row>
								{/each}
							</Table.Body>
						{/if}
					</Table.Root>
				</TableContainer>
			</Card.Content>
		</Card.Root>

		<PaginationFooter
			currentPage={page}
			{totalPages}
			{pageSize}
			totalItems={total}
			onPageChange={handlePageChange}
			onPageSizeChange={handlePageSizeChange}
			loading={resultsLoading}
			itemLabel="run"
		/>

		{#if incidents.length > 0}
			<Card.Root class="gap-3 pt-2">
				<Card.Header class="border-b pl-3 [.border-b]:pb-2">
					<Card.Title class="flex items-center gap-2 text-base font-medium">
						<TriangleAlert class="h-3.5 w-3.5 text-muted-foreground" />
						<span class="text-sm font-[400] text-muted-foreground">Incidents</span>
					</Card.Title>
				</Card.Header>
				<Card.Content class="pb-4">
					<div class="space-y-2">
						{#each incidents as incident, __index (__index)}
							<div
								class="flex flex-col gap-2 rounded-md border p-3 text-sm sm:flex-row sm:items-start sm:justify-between sm:gap-4"
							>
								<div class="min-w-0">
									{#if incident.title}
										<div class="font-medium">{incident.title}</div>
									{/if}
									<div class={incident.title ? 'text-xs text-muted-foreground' : 'font-medium'}>
										{formatDateTime(incident.startedAt, { format: 'short' })}
										{#if incident.resolvedAt}
											<span class="text-muted-foreground">
												→ {formatDateTime(incident.resolvedAt, { format: 'short' })}
											</span>
										{/if}
									</div>
									{#if incident.errorMessage}
										<div class="mt-1 text-xs break-all text-muted-foreground">
											{incident.errorMessage}
										</div>
									{/if}
								</div>
								<div class="flex shrink-0 items-center gap-2">
									{#if incident.postMortemId}
										<a
											class="inline-flex items-center gap-1 text-xs text-primary underline-offset-2 hover:underline"
											href={resolve(`/monitors/post-mortems/${incident.postMortemId}`)}
										>
											<FileText class="h-3.5 w-3.5" />
											Post-mortem
										</a>
									{:else if canWritePostMortems}
										<button
											type="button"
											class="inline-flex cursor-pointer items-center gap-1 text-xs text-muted-foreground underline-offset-2 hover:text-primary hover:underline"
											onclick={() => {
												newPostMortemIncident = incident;
												newPostMortemOpen = true;
											}}
										>
											<FileText class="h-3.5 w-3.5" />
											Write post-mortem
										</button>
									{/if}
									{#if incident.resolvedAt}
										<StatusPill tone="success" label="Resolved" />
									{:else}
										<StatusPill tone="danger" label="Ongoing" />
									{/if}
								</div>
							</div>
						{/each}
					</div>
				</Card.Content>
			</Card.Root>
		{/if}
	</div>

	<MonitorSheet bind:open={editOpen} {check} onSaved={() => loadAll()} />

	<NewPostMortemDialog
		bind:open={newPostMortemOpen}
		incidentId={newPostMortemIncident?.id ?? null}
		incidentLabel={newPostMortemIncident
			? incidentDisplayTitle({ title: newPostMortemIncident.title, checkName: check?.name ?? null })
			: ''}
	/>

	<ConfirmDeleteDialog
		bind:open={deleteOpen}
		entity="Monitor"
		description={`This permanently deletes "${check.name}" along with its run history and incidents.`}
		loading={deleting}
		onConfirm={deleteCheck}
	/>

	<AlertDialog.Root bind:open={outputOpen}>
		<AlertDialog.Content class="sm:max-w-3xl">
			<AlertDialog.Header>
				<AlertDialog.Title>Run output</AlertDialog.Title>
				<AlertDialog.Description>
					Playwright output captured from the failed run.
				</AlertDialog.Description>
			</AlertDialog.Header>
			{#if outputLoading}
				<div class="flex h-40 items-center justify-center">
					<LoadingCircle size="lg" />
				</div>
			{:else}
				<pre
					class="max-h-[60vh] overflow-auto rounded-md border bg-muted/50 p-3 font-mono text-xs whitespace-pre-wrap">{outputText}</pre>
			{/if}
			<AlertDialog.Footer>
				<AlertDialog.Cancel>Close</AlertDialog.Cancel>
			</AlertDialog.Footer>
		</AlertDialog.Content>
	</AlertDialog.Root>

	<AlertDialog.Root bind:open={screenshotOpen}>
		<AlertDialog.Content class="sm:max-w-3xl">
			<AlertDialog.Header>
				<AlertDialog.Title>Failure screenshot</AlertDialog.Title>
				<AlertDialog.Description>
					Screenshot captured at the point of failure.
				</AlertDialog.Description>
			</AlertDialog.Header>
			{#if screenshotLoading}
				<div class="flex h-40 items-center justify-center">
					<LoadingCircle size="lg" />
				</div>
			{:else if screenshotUrl}
				<img
					src={screenshotUrl}
					alt="Failure screenshot"
					class="max-h-[60vh] w-full rounded-md border object-contain"
				/>
			{/if}
			<AlertDialog.Footer>
				<AlertDialog.Cancel>Close</AlertDialog.Cancel>
			</AlertDialog.Footer>
		</AlertDialog.Content>
	</AlertDialog.Root>

	<AlertDialog.Root bind:open={errorOpen}>
		<AlertDialog.Content class="sm:max-w-3xl">
			<AlertDialog.Header>
				<AlertDialog.Title>Run error</AlertDialog.Title>
				<AlertDialog.Description>Full error message reported by this run.</AlertDialog.Description>
			</AlertDialog.Header>
			<pre
				class="max-h-[60vh] overflow-auto rounded-md border bg-muted/50 p-3 font-mono text-xs break-all whitespace-pre-wrap">{errorText}</pre>
			<AlertDialog.Footer>
				<AlertDialog.Cancel>Close</AlertDialog.Cancel>
			</AlertDialog.Footer>
		</AlertDialog.Content>
	</AlertDialog.Root>
{/if}
