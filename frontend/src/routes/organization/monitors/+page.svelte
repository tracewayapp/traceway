<script lang="ts">
	import { onDestroy, onMount, untrack } from 'svelte';
	import { browser } from '$app/environment';
	import { api } from '$lib/api';
	import { organizationContext } from '$lib/state/organization-context.svelte';
	import { getErrorMessage } from '$lib/utils/errors';
	import { formatDurationMs, formatRelativeTimeAgo } from '$lib/utils/formatters';
	import { createRowClickHandler } from '$lib/utils/navigation';
	import { uptimeClass } from '$lib/utils/uptime';
	import { TimeRangeState } from '$lib/utils/time-range-state.svelte';
	import { updateUrl } from '$lib/utils/url-params';
	import * as Table from '$lib/components/ui/table';
	import { Badge } from '$lib/components/ui/badge';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { SearchBar } from '$lib/components/ui/search-bar';
	import { TracewayTableHeader } from '$lib/components/ui/traceway-table-header';
	import { TableEmptyState } from '$lib/components/ui/table-empty-state';
	import { PaginationFooter } from '$lib/components/ui/pagination-footer';
	import { TimeRangePicker } from '$lib/components/ui/time-range-picker';
	import PageHeader from '$lib/components/traceway/page-header.svelte';
	import TableContainer from '$lib/components/traceway/table-container.svelte';
	import ErrorRetryBox from '$lib/components/traceway/error-retry-box.svelte';
	import CheckStatusBadge from '$lib/components/synthetics/check-status-badge.svelte';
	import MonitorTypeBadge from '$lib/components/synthetics/monitor-type-badge.svelte';
	import CountPill from '../count-pill.svelte';
	import PartialNotice from '../partial-notice.svelte';
	import {
		incidentDisplayTitle,
		type CheckOverview,
		type OrgIncident
	} from '$lib/state/monitors.svelte';
	import {
		getSortState,
		setSortState,
		handleSortClick,
		type SortDirection
	} from '$lib/utils/sort-storage';

	interface OrgMonitorRow extends CheckOverview {
		projectName: string;
	}

	type SortField = 'name' | 'project' | 'status' | 'uptime' | 'latency' | 'last_run';

	const typeOptions = [
		{ value: '', label: 'All types' },
		{ value: 'http', label: 'HTTP' },
		{ value: 'tcp', label: 'TCP' },
		{ value: 'browser', label: 'Browser' }
	];

	function readParams() {
		if (!browser) return { search: '', incidentSearch: '' };
		const params = new URLSearchParams(window.location.search);
		return {
			search: params.get('search') || '',
			incidentSearch: params.get('incidentSearch') || ''
		};
	}

	const initialParams = readParams();
	let searchQuery = $state(initialParams.search);
	let typeFilter = $state('');
	let incidentSearch = $state(initialParams.incidentSearch);

	const range = new TimeRangeState('3M');

	let checks = $state<OrgMonitorRow[]>([]);
	let checksLoading = $state(true);
	let checksError = $state('');
	let checksPartial = $state(false);
	let checksGeneration = 0;

	let incidents = $state<OrgIncident[]>([]);
	let incidentsLoading = $state(true);
	let incidentsError = $state('');
	let incidentsPage = $state(1);
	let incidentsPageSize = $state(20);
	let incidentsTotal = $state(0);
	let incidentsTotalPages = $state(0);
	let incidentsGeneration = 0;
	let activeOrganizationId: number | null = null;

	const SORT_STORAGE_KEY = 'org-monitors';
	const initialSort = getSortState(SORT_STORAGE_KEY, { field: 'status', direction: 'asc' });
	let orderBy = $state<SortField>(initialSort.field as SortField);
	let sortDirection = $state<SortDirection>(initialSort.direction);

	function syncUrl(pushToHistory: boolean) {
		updateUrl(
			{
				...range.urlParams(),
				search: searchQuery.trim() || null,
				incidentSearch: incidentSearch.trim() || null
			},
			{ pushToHistory }
		);
	}

	async function loadChecks(organizationId: number) {
		const generation = ++checksGeneration;
		checksLoading = true;
		checksError = '';
		checksPartial = false;
		try {
			const response = await api.get(`/organizations/${organizationId}/overview/monitors`);
			if (generation !== checksGeneration) return;
			checks = response.checks || [];
			checksPartial = response.partial === true;
		} catch (e) {
			if (generation !== checksGeneration) return;
			checksError = getErrorMessage(e) || 'Failed to load monitors';
		} finally {
			if (generation === checksGeneration) checksLoading = false;
		}
	}

	async function loadIncidents(organizationId: number, pushToHistory: boolean) {
		const generation = ++incidentsGeneration;
		incidentsLoading = true;
		incidentsError = '';
		range.resolvePreset();
		syncUrl(pushToHistory);
		try {
			const response = await api.post(`/organizations/${organizationId}/overview/incidents`, {
				search: incidentSearch.trim(),
				fromDate: range.fromUTC(),
				toDate: range.toUTC(),
				pagination: { page: incidentsPage, pageSize: incidentsPageSize }
			});
			if (generation !== incidentsGeneration) return;
			incidents = response.data || [];
			incidentsTotal = response.pagination?.total || 0;
			incidentsTotalPages = response.pagination?.totalPages || 0;
		} catch (e) {
			if (generation !== incidentsGeneration) return;
			incidentsError = getErrorMessage(e) || 'Failed to load incidents';
		} finally {
			if (generation === incidentsGeneration) incidentsLoading = false;
		}
	}

	function reloadIncidents(pushToHistory: boolean) {
		const organizationId = organizationContext.organizationId;
		if (organizationId !== null) void loadIncidents(organizationId, pushToHistory);
	}

	function retryChecks() {
		const organizationId = organizationContext.organizationId;
		if (organizationId !== null) void loadChecks(organizationId);
	}

	function handleSearch() {
		syncUrl(true);
	}

	function handleIncidentSearch() {
		incidentsPage = 1;
		reloadIncidents(true);
	}

	function handleTimeRangeChange(
		from: Parameters<TimeRangeState['apply']>[0],
		to: Parameters<TimeRangeState['apply']>[1],
		preset: string | null
	) {
		range.apply(from, to, preset);
		incidentsPage = 1;
		reloadIncidents(true);
	}

	function handleIncidentsPageChange(newPage: number) {
		if (newPage >= 1 && newPage <= incidentsTotalPages) {
			incidentsPage = newPage;
			reloadIncidents(true);
		}
	}

	function handleIncidentsPageSizeChange(newSize: number) {
		incidentsPageSize = newSize;
		incidentsPage = 1;
		reloadIncidents(true);
	}

	function handlePopState() {
		range.readUrl();
		const params = readParams();
		searchQuery = params.search;
		incidentSearch = params.incidentSearch;
		incidentsPage = 1;
		reloadIncidents(false);
	}

	$effect(() => {
		const organizationId = organizationContext.organizationId;
		if (organizationId === null || organizationId === activeOrganizationId) return;
		activeOrganizationId = organizationId;
		untrack(() => {
			incidentsPage = 1;
			void loadChecks(organizationId);
			void loadIncidents(organizationId, false);
		});
	});

	onMount(() => {
		window.addEventListener('popstate', handlePopState);
	});

	onDestroy(() => {
		if (typeof window !== 'undefined') {
			window.removeEventListener('popstate', handlePopState);
		}
	});

	function handleSort(field: SortField) {
		const newSort = handleSortClick(field, orderBy, sortDirection);
		orderBy = newSort.field as SortField;
		sortDirection = newSort.direction;
		setSortState(SORT_STORAGE_KEY, newSort);
	}

	function uptimePct(check: OrgMonitorRow): number | null {
		const agg = check.aggregates;
		if (!agg || agg.total === 0) return null;
		const measured = agg.total - agg.missed;
		if (measured === 0) return null;
		return (agg.up / measured) * 100;
	}

	function target(check: OrgMonitorRow): string {
		if (check.checkType === 'http')
			return `${check.config?.method || 'GET'} ${check.config?.url ?? ''}`;
		if (check.checkType === 'tcp') return `${check.config?.host ?? ''}:${check.config?.port ?? ''}`;
		return 'Playwright script';
	}

	const statusRank: Record<string, number> = { down: 0, unknown: 1, up: 2 };

	const downCount = $derived(
		checks.filter((check) => check.enabled && check.currentStatus === 'down').length
	);

	const visibleChecks = $derived.by(() => {
		const query = searchQuery.trim().toLowerCase();
		let filtered = typeFilter ? checks.filter((c) => c.checkType === typeFilter) : checks;
		if (query) {
			filtered = filtered.filter((c) =>
				[c.name, c.projectName, c.checkType, target(c)].join(' ').toLowerCase().includes(query)
			);
		}
		const direction = sortDirection === 'asc' ? 1 : -1;
		return [...filtered].sort((a, b) => {
			switch (orderBy) {
				case 'project':
					return direction * a.projectName.localeCompare(b.projectName);
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
</script>

<div class="space-y-8">
	<section class="space-y-4">
		<PageHeader
			title="Monitors"
			description="Every monitor in the organization with its current status, 30-day uptime, and average latency."
		>
			{#snippet trailing()}
				{#if !checksLoading && !checksError}
					<CountPill count={checks.length} />
					{#if downCount > 0}
						<CountPill count={downCount} label="down" tone="danger" />
					{/if}
				{/if}
			{/snippet}
		</PageHeader>

		<SearchBar
			placeholder="Search monitors..."
			bind:value={searchQuery}
			bind:typeValue={typeFilter}
			{typeOptions}
			onSearch={handleSearch}
			disabled={checksLoading}
		/>

		{#if checksPartial}
			<PartialNotice />
		{/if}
		{#if checksLoading}
			<div class="flex h-48 items-center justify-center">
				<LoadingCircle size="xlg" />
			</div>
		{:else if checksError}
			<ErrorRetryBox message={checksError} onRetry={retryChecks} />
		{:else if checks.length === 0}
			<div class="rounded-md border py-16 text-center text-sm text-muted-foreground">
				No monitors in this organization yet. Create monitors from a project's Monitors page.
			</div>
		{:else if visibleChecks.length === 0}
			<div class="rounded-md border py-16 text-center text-sm text-muted-foreground">
				No monitors match the search.
			</div>
		{:else}
			<TableContainer minWidth="920px">
				<Table.Root>
					<Table.Header>
						<Table.Row>
							<TracewayTableHeader
								label="Name"
								sortField="name"
								currentSortField={orderBy}
								{sortDirection}
								onSort={(field) => handleSort(field as SortField)}
							/>
							<TracewayTableHeader label="Type" class="w-[100px]" />
							<TracewayTableHeader
								label="Project"
								sortField="project"
								currentSortField={orderBy}
								{sortDirection}
								onSort={(field) => handleSort(field as SortField)}
								class="w-[160px]"
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
								tooltip="Share of successful probes over the last 30 days (missed probes excluded)"
								sortField="uptime"
								currentSortField={orderBy}
								{sortDirection}
								onSort={(field) => handleSort(field as SortField)}
								class="w-[160px]"
							/>
							<TracewayTableHeader
								label="Avg latency"
								tooltip="Average probe latency over the last 30 days"
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
								class="w-[130px]"
								align="right"
							/>
						</Table.Row>
					</Table.Header>
					<Table.Body>
						{#each visibleChecks as check (check.id)}
							{@const uptime = uptimePct(check)}
							<Table.Row
								class="cursor-pointer"
								onclick={createRowClickHandler(
									`/monitors/${check.id}?projectId=${check.projectId}`
								)}
							>
								<Table.Cell class="font-medium whitespace-normal">
									{check.name}
									<div
										class="mt-0.5 max-w-[420px] truncate font-mono text-xs font-normal text-muted-foreground"
									>
										{target(check)}
									</div>
								</Table.Cell>
								<Table.Cell>
									<MonitorTypeBadge type={check.checkType} />
								</Table.Cell>
								<Table.Cell class="text-sm text-muted-foreground">{check.projectName}</Table.Cell>
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
									{check.lastRunAt ? formatRelativeTimeAgo(check.lastRunAt) : 'never'}
								</Table.Cell>
							</Table.Row>
						{/each}
					</Table.Body>
				</Table.Root>
			</TableContainer>
		{/if}
	</section>

	<section class="space-y-4">
		<div class="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
			<div>
				<div class="flex items-center gap-2">
					<h2 class="text-lg font-semibold">Incidents</h2>
					{#if !incidentsLoading && !incidentsError}
						<CountPill count={incidentsTotal} />
					{/if}
				</div>
				<p class="mt-0.5 text-sm text-muted-foreground">
					Across monitors and status pages, for incidents open at any point in the selected range.
				</p>
			</div>
			<TimeRangePicker
				bind:fromDate={range.fromDate}
				bind:toDate={range.toDate}
				bind:fromTime={range.fromTime}
				bind:toTime={range.toTime}
				bind:preset={range.preset}
				onApply={handleTimeRangeChange}
			/>
		</div>

		<SearchBar
			placeholder="Search incidents by title, monitor, or status page..."
			bind:value={incidentSearch}
			onSearch={handleIncidentSearch}
			disabled={incidentsLoading}
		/>

		<TableContainer minWidth="720px">
			<Table.Root>
				{#if incidentsLoading}
					<Table.Body>
						<Table.Row>
							<Table.Cell colspan={4} class="h-32">
								<div class="flex h-full items-center justify-center">
									<LoadingCircle size="lg" />
								</div>
							</Table.Cell>
						</Table.Row>
					</Table.Body>
				{:else if incidentsError}
					<Table.Body>
						<Table.Row>
							<Table.Cell colspan={4} class="h-32">
								<div class="flex h-full flex-col items-center justify-center gap-3">
									<p class="text-sm text-destructive">{incidentsError}</p>
									<button
										class="text-sm font-medium text-primary hover:underline"
										onclick={() => reloadIncidents(false)}>Retry</button
									>
								</div>
							</Table.Cell>
						</Table.Row>
					</Table.Body>
				{:else if incidents.length === 0}
					<Table.Body>
						<TableEmptyState colspan={4} message="No incidents in the selected range." />
					</Table.Body>
				{:else}
					<Table.Header>
						<Table.Row>
							<TracewayTableHeader label="Incident" />
							<TracewayTableHeader label="Monitor" class="w-[200px]" />
							<TracewayTableHeader label="Started" class="w-[150px]" align="right" />
							<TracewayTableHeader label="Resolved" class="w-[150px]" align="right" />
						</Table.Row>
					</Table.Header>
					<Table.Body>
						{#each incidents as incident (incident.id)}
							<Table.Row
								class={incident.checkId !== null && incident.projectId !== null
									? 'cursor-pointer'
									: ''}
								onclick={incident.checkId !== null && incident.projectId !== null
									? createRowClickHandler(
											`/monitors/${incident.checkId}?projectId=${incident.projectId}`
										)
									: undefined}
							>
								<Table.Cell class="max-w-[420px] py-3">
									<div class="min-w-0">
										<div class="truncate font-medium">{incidentDisplayTitle(incident)}</div>
										{#if incident.errorMessage}
											<div class="truncate text-xs text-muted-foreground">
												{incident.errorMessage}
											</div>
										{/if}
									</div>
								</Table.Cell>
								<Table.Cell class="text-sm text-muted-foreground">
									{incident.checkName || incident.statusPageName || '—'}
								</Table.Cell>
								<Table.Cell class="text-right text-sm text-muted-foreground">
									{formatRelativeTimeAgo(incident.startedAt)}
								</Table.Cell>
								<Table.Cell class="text-right text-sm">
									{#if incident.resolvedAt === null}
										<Badge variant="destructive">ongoing</Badge>
									{:else}
										<span class="text-muted-foreground">
											{formatRelativeTimeAgo(incident.resolvedAt)}
										</span>
									{/if}
								</Table.Cell>
							</Table.Row>
						{/each}
					</Table.Body>
				{/if}
			</Table.Root>
		</TableContainer>

		<PaginationFooter
			currentPage={incidentsPage}
			totalPages={incidentsTotalPages}
			pageSize={incidentsPageSize}
			totalItems={incidentsTotal}
			onPageChange={handleIncidentsPageChange}
			onPageSizeChange={handleIncidentsPageSizeChange}
			loading={incidentsLoading}
			itemLabel="incident"
		/>
	</section>
</div>
