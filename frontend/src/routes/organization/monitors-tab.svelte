<script lang="ts">
	import { SvelteMap } from 'svelte/reactivity';
	import { api } from '$lib/api';
	import { getErrorMessage } from '$lib/utils/errors';
	import { formatDurationMs, formatRelativeTimeAgo } from '$lib/utils/formatters';
	import { createRowClickHandler } from '$lib/utils/navigation';
	import { uptimeClass } from '$lib/utils/uptime';
	import * as Table from '$lib/components/ui/table';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { TracewayTableHeader } from '$lib/components/ui/traceway-table-header';
	import { Badge } from '$lib/components/ui/badge';
	import TableContainer from '$lib/components/traceway/table-container.svelte';
	import ErrorRetryBox from '$lib/components/traceway/error-retry-box.svelte';
	import CheckStatusBadge from '$lib/components/synthetics/check-status-badge.svelte';
	import MonitorTypeBadge from '$lib/components/synthetics/monitor-type-badge.svelte';
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

	type SortField = 'name' | 'project' | 'status' | 'uptime' | 'latency';

	let { organizationId }: { organizationId: number } = $props();

	let checks = $state<OrgMonitorRow[]>([]);
	let checksLoading = $state(true);
	let checksError = $state('');

	let incidents = $state<OrgIncident[]>([]);
	let incidentsLoading = $state(true);
	let incidentsError = $state('');
	let loadGeneration = 0;

	const SORT_STORAGE_KEY = 'org-monitors';
	const initialSort = getSortState(SORT_STORAGE_KEY, { field: 'status', direction: 'asc' });
	let orderBy = $state<SortField>(initialSort.field as SortField);
	let sortDirection = $state<SortDirection>(initialSort.direction);

	async function loadChecks(orgId: number, generation: number) {
		checksLoading = true;
		checksError = '';
		try {
			const response = await api.get(`/organizations/${orgId}/overview/monitors`);
			if (generation !== loadGeneration) return;
			checks = response.checks || [];
		} catch (e) {
			if (generation !== loadGeneration) return;
			checksError = getErrorMessage(e) || 'Failed to load monitors';
		} finally {
			if (generation === loadGeneration) checksLoading = false;
		}
	}

	async function loadIncidents(orgId: number, generation: number) {
		incidentsLoading = true;
		incidentsError = '';
		try {
			const response = await api.get(`/organizations/${orgId}/incidents`);
			if (generation !== loadGeneration) return;
			incidents = response.incidents || [];
		} catch (e) {
			if (generation !== loadGeneration) return;
			incidentsError = getErrorMessage(e) || 'Failed to load incidents';
		} finally {
			if (generation === loadGeneration) incidentsLoading = false;
		}
	}

	$effect(() => {
		const orgId = organizationId;
		const generation = ++loadGeneration;
		loadChecks(orgId, generation);
		loadIncidents(orgId, generation);
	});

	function retryChecks() {
		loadChecks(organizationId, ++loadGeneration);
	}

	function retryIncidents() {
		loadIncidents(organizationId, ++loadGeneration);
	}

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

	const statusRank: Record<string, number> = { down: 0, unknown: 1, up: 2 };

	const lastIncidentByCheck = $derived.by(() => {
		const byCheck = new SvelteMap<number, OrgIncident>();
		for (const incident of incidents) {
			if (incident.checkId === null) continue;
			const existing = byCheck.get(incident.checkId);
			if (!existing || new Date(incident.startedAt) > new Date(existing.startedAt)) {
				byCheck.set(incident.checkId, incident);
			}
		}
		return byCheck;
	});

	const sortedChecks = $derived.by(() => {
		const direction = sortDirection === 'asc' ? 1 : -1;
		return [...checks].sort((a, b) => {
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
				default:
					return direction * a.name.localeCompare(b.name);
			}
		});
	});
</script>

<div class="space-y-6">
	<section class="space-y-3">
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
							<TracewayTableHeader label="Last incident" class="w-[160px]" align="right" />
						</Table.Row>
					</Table.Header>
					<Table.Body>
						{#each sortedChecks as check (check.id)}
							{@const uptime = uptimePct(check)}
							{@const lastIncident = lastIncidentByCheck.get(check.id)}
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
									{#if lastIncident}
										{#if lastIncident.resolvedAt === null}
											<Badge variant="destructive">ongoing</Badge>
										{:else}
											{formatRelativeTimeAgo(lastIncident.startedAt)}
										{/if}
									{:else}
										—
									{/if}
								</Table.Cell>
							</Table.Row>
						{/each}
					</Table.Body>
				</Table.Root>
			</TableContainer>
		{/if}
	</section>

	<section class="space-y-3">
		<h2 class="text-lg font-semibold">Recent incidents</h2>
		{#if incidentsLoading}
			<div class="flex h-32 items-center justify-center">
				<LoadingCircle size="lg" />
			</div>
		{:else if incidentsError}
			<ErrorRetryBox message={incidentsError} onRetry={retryIncidents} />
		{:else if incidents.length === 0}
			<div class="rounded-md border py-8 text-center text-sm text-muted-foreground">
				No incidents in the last 90 days.
			</div>
		{:else}
			<TableContainer minWidth="720px">
				<Table.Root>
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
				</Table.Root>
			</TableContainer>
		{/if}
	</section>
</div>
