<script lang="ts">
	import { api } from '$lib/api';
	import { getErrorMessage } from '$lib/utils/errors';
	import { formatRelativeTimeAgo } from '$lib/utils/formatters';
	import { createRowClickHandler } from '$lib/utils/navigation';
	import { getServerColor } from '$lib/utils/server-colors';
	import * as Table from '$lib/components/ui/table';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { TracewayTableHeader } from '$lib/components/ui/traceway-table-header';
	import TableContainer from '$lib/components/traceway/table-container.svelte';
	import StatRow from '$lib/components/traceway/stat-row.svelte';
	import StatTile from '$lib/components/traceway/stat-tile.svelte';
	import PageBadges from '$lib/components/traceway/page-badges.svelte';
	import Sparkline from '$lib/components/dashboard/sparkline.svelte';
	import type { MetricTrendPoint } from '$lib/types/dashboard';
	import type { OncallPage } from '$lib/state/oncall.svelte';

	interface OrgServerRow {
		serverName: string;
		projectId: string;
		projectName: string;
		cpuPct: number;
		lastReportedAt: string;
		trend: { timestamp: string; value: number }[];
	}

	interface OrgIssueRow {
		exceptionHash: string;
		stackTrace: string;
		lastSeen: string;
		firstSeen: string;
		count: number;
		projectId: string;
		projectName: string;
	}

	interface OrgPageRow extends OncallPage {
		projectName: string;
	}

	let { organizationId }: { organizationId: number } = $props();

	// A server counts as live when it reported within the last 3 minutes;
	// anything older (up to the 30-minute listing window) shows as stale.
	const FRESH_MS = 3 * 60 * 1000;

	let servers = $state<OrgServerRow[]>([]);
	let serversLoading = $state(true);
	let serversError = $state('');

	let issues = $state<OrgIssueRow[]>([]);
	let totalGroups = $state(0);
	let issuesLoading = $state(true);
	let issuesError = $state('');

	let pages = $state<OrgPageRow[]>([]);
	let openPagesCount = $state(0);
	let downMonitorsCount = $state(0);
	let pagesLoading = $state(true);
	let pagesError = $state('');

	let now = $state(Date.now());

	async function loadServers(orgId: number) {
		serversLoading = true;
		serversError = '';
		try {
			const response = await api.get(`/organizations/${orgId}/overview/servers`);
			servers = response.servers || [];
			now = Date.now();
		} catch (e) {
			serversError = getErrorMessage(e) || 'Failed to load server health';
		} finally {
			serversLoading = false;
		}
	}

	async function loadIssues(orgId: number) {
		issuesLoading = true;
		issuesError = '';
		try {
			const response = await api.get(`/organizations/${orgId}/overview/issues`);
			issues = response.issues || [];
			totalGroups = response.totalGroups || 0;
		} catch (e) {
			issuesError = getErrorMessage(e) || 'Failed to load issues';
		} finally {
			issuesLoading = false;
		}
	}

	async function loadPages(orgId: number) {
		pagesLoading = true;
		pagesError = '';
		try {
			const response = await api.get(`/organizations/${orgId}/overview/pages`);
			pages = response.pages || [];
			openPagesCount = response.openPagesCount || 0;
			downMonitorsCount = response.downMonitorsCount || 0;
		} catch (e) {
			pagesError = getErrorMessage(e) || 'Failed to load on-call pages';
		} finally {
			pagesLoading = false;
		}
	}

	$effect(() => {
		const orgId = organizationId;
		loadServers(orgId);
		loadIssues(orgId);
		loadPages(orgId);
	});

	function isFresh(server: OrgServerRow): boolean {
		return now - new Date(server.lastReportedAt).getTime() <= FRESH_MS;
	}

	const freshCount = $derived(servers.filter(isFresh).length);
	const staleCount = $derived(servers.length - freshCount);
	const allServerNames = $derived(servers.map((s) => s.serverName));

	function toTrendPoints(trend: OrgServerRow['trend']): MetricTrendPoint[] {
		return (trend || []).map((p) => ({ timestamp: new Date(p.timestamp), value: p.value }));
	}

	function issueTitle(row: OrgIssueRow): { type: string; message: string } {
		const firstLine = row.stackTrace.split('\n')[0];
		const colonIndex = firstLine.indexOf(':');
		return {
			type: colonIndex > 0 ? firstLine.slice(0, colonIndex) : firstLine,
			message: colonIndex > 0 ? firstLine.slice(colonIndex + 1).trim() : ''
		};
	}
</script>

<div class="space-y-6">
	<StatRow columns={5}>
		<StatTile
			label="Servers reporting"
			value={serversLoading ? '—' : freshCount}
			sub="reported in the last 3 min"
			valueClass={!serversLoading && freshCount > 0 ? 'text-green-600 dark:text-green-400' : ''}
		/>
		<StatTile
			label="Servers stale"
			value={serversLoading ? '—' : staleCount}
			sub="quiet for over 3 min"
			valueClass={!serversLoading && staleCount > 0 ? 'text-amber-600 dark:text-amber-400' : ''}
		/>
		<StatTile
			label="Issues (24h)"
			value={issuesLoading ? '—' : totalGroups}
			sub="across all projects"
		/>
		<StatTile
			label="Open pages"
			value={pagesLoading ? '—' : openPagesCount}
			sub="awaiting acknowledgement"
			valueClass={!pagesLoading && openPagesCount > 0 ? 'text-red-600 dark:text-red-400' : ''}
		/>
		<StatTile
			label="Monitors down"
			value={pagesLoading ? '—' : downMonitorsCount}
			sub="failing right now"
			valueClass={!pagesLoading && downMonitorsCount > 0 ? 'text-red-600 dark:text-red-400' : ''}
		/>
	</StatRow>

	<section class="space-y-3">
		<h2 class="text-lg font-semibold">Server health</h2>
		{#if serversLoading}
			<div class="flex h-32 items-center justify-center">
				<LoadingCircle size="lg" />
			</div>
		{:else if serversError}
			<div class="rounded-md border py-8 text-center text-sm text-red-500">{serversError}</div>
		{:else if servers.length === 0}
			<div class="rounded-md border py-8 text-center text-sm text-muted-foreground">
				No servers reported metrics in the last 30 minutes.
			</div>
		{:else}
			<TableContainer minWidth="760px">
				<Table.Root>
					<Table.Header>
						<Table.Row>
							<TracewayTableHeader label="Server" />
							<TracewayTableHeader label="Project" class="w-[180px]" />
							<TracewayTableHeader label="CPU" class="w-[90px]" align="right" />
							<TracewayTableHeader label="CPU (30 min)" class="w-[200px]" />
							<TracewayTableHeader label="Last reported" class="w-[150px]" align="right" />
						</Table.Row>
					</Table.Header>
					<Table.Body>
						{#each servers as server (server.projectId + server.serverName)}
							{@const fresh = isFresh(server)}
							<Table.Row
								class="cursor-pointer"
								onclick={createRowClickHandler(`/?projectId=${server.projectId}`)}
							>
								<Table.Cell class="font-medium">
									<div class="flex items-center gap-2">
										<span
											class="h-2 w-2 shrink-0 rounded-full {fresh
												? 'bg-green-500'
												: 'bg-muted-foreground/40'}"
											title={fresh
												? 'Reported in the last 3 minutes'
												: 'No report in over 3 minutes'}
										></span>
										<span class="font-mono text-sm">{server.serverName}</span>
									</div>
								</Table.Cell>
								<Table.Cell class="text-sm text-muted-foreground">{server.projectName}</Table.Cell>
								<Table.Cell class="text-right font-mono text-sm tabular-nums">
									{server.cpuPct.toFixed(1)}%
								</Table.Cell>
								<Table.Cell>
									<Sparkline
										data={toTrendPoints(server.trend)}
										color={getServerColor(server.serverName, allServerNames)}
										height={28}
									/>
								</Table.Cell>
								<Table.Cell class="text-right text-sm text-muted-foreground">
									{formatRelativeTimeAgo(server.lastReportedAt)}
								</Table.Cell>
							</Table.Row>
						{/each}
					</Table.Body>
				</Table.Root>
			</TableContainer>
		{/if}
	</section>

	<section class="space-y-3">
		<h2 class="text-lg font-semibold">Issues across projects</h2>
		{#if issuesLoading}
			<div class="flex h-32 items-center justify-center">
				<LoadingCircle size="lg" />
			</div>
		{:else if issuesError}
			<div class="rounded-md border py-8 text-center text-sm text-red-500">{issuesError}</div>
		{:else if issues.length === 0}
			<div class="rounded-md border py-8 text-center text-sm text-muted-foreground">
				No issues in the last 24 hours. All quiet.
			</div>
		{:else}
			<TableContainer minWidth="720px">
				<Table.Root>
					<Table.Header>
						<Table.Row>
							<TracewayTableHeader label="Issue" />
							<TracewayTableHeader label="Project" class="w-[180px]" />
							<TracewayTableHeader label="Events" class="w-[90px]" align="right" />
							<TracewayTableHeader label="Last seen" class="w-[150px]" align="right" />
						</Table.Row>
					</Table.Header>
					<Table.Body>
						{#each issues as issue (issue.projectId + issue.exceptionHash)}
							{@const title = issueTitle(issue)}
							<Table.Row
								class="group cursor-pointer"
								onclick={createRowClickHandler(
									`/issues/${issue.exceptionHash}?projectId=${issue.projectId}`
								)}
							>
								<Table.Cell class="max-w-[520px] py-3">
									<div class="min-w-0">
										<div
											class="truncate text-[15px]/6 font-semibold text-foreground group-hover:text-primary dark:group-hover:text-blue-300"
										>
											{title.type}
										</div>
										{#if title.message}
											<div class="truncate text-sm text-muted-foreground">{title.message}</div>
										{/if}
									</div>
								</Table.Cell>
								<Table.Cell class="text-sm text-muted-foreground">{issue.projectName}</Table.Cell>
								<Table.Cell class="text-right font-semibold tabular-nums">
									{issue.count.toLocaleString()}
								</Table.Cell>
								<Table.Cell class="text-right text-sm text-muted-foreground">
									{formatRelativeTimeAgo(issue.lastSeen)}
								</Table.Cell>
							</Table.Row>
						{/each}
					</Table.Body>
				</Table.Root>
			</TableContainer>
		{/if}
	</section>

	<section class="space-y-3">
		<h2 class="text-lg font-semibold">Open on-call pages</h2>
		{#if pagesLoading}
			<div class="flex h-32 items-center justify-center">
				<LoadingCircle size="lg" />
			</div>
		{:else if pagesError}
			<div class="rounded-md border py-8 text-center text-sm text-red-500">{pagesError}</div>
		{:else if pages.length === 0}
			<div class="rounded-md border py-8 text-center text-sm text-muted-foreground">
				No open pages. Nobody is being paged right now.
			</div>
		{:else}
			<TableContainer minWidth="760px">
				<Table.Root>
					<Table.Header>
						<Table.Row>
							<TracewayTableHeader label="Page" />
							<TracewayTableHeader label="Project" class="w-[180px]" />
							<TracewayTableHeader label="Severity" class="w-[160px]" />
							<TracewayTableHeader label="Status" class="w-[130px]" />
							<TracewayTableHeader label="Opened" class="w-[150px]" align="right" />
						</Table.Row>
					</Table.Header>
					<Table.Body>
						{#each pages as oncallPage (oncallPage.id)}
							<Table.Row
								class="cursor-pointer"
								onclick={createRowClickHandler(
									`/on-call?tab=pages&page=${oncallPage.id}&projectId=${oncallPage.projectId}`
								)}
							>
								<Table.Cell class="max-w-[420px] py-3">
									<div class="min-w-0">
										<div class="truncate font-medium">{oncallPage.subject}</div>
										{#if oncallPage.ruleName}
											<div class="truncate text-xs text-muted-foreground">
												{oncallPage.ruleName}
											</div>
										{/if}
									</div>
								</Table.Cell>
								<Table.Cell class="text-sm text-muted-foreground">
									{oncallPage.projectName || '—'}
								</Table.Cell>
								<Table.Cell>
									<div class="flex flex-wrap gap-1">
										<PageBadges
											severity={oncallPage.severity}
											urgency={oncallPage.urgency}
											fallback
										/>
									</div>
								</Table.Cell>
								<Table.Cell>
									<PageBadges status={oncallPage.status} />
								</Table.Cell>
								<Table.Cell class="text-right text-sm text-muted-foreground">
									{formatRelativeTimeAgo(oncallPage.createdAt)}
								</Table.Cell>
							</Table.Row>
						{/each}
					</Table.Body>
				</Table.Root>
			</TableContainer>
		{/if}
	</section>
</div>
