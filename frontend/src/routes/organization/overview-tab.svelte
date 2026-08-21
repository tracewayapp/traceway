<script lang="ts">
	import { resolve } from '$app/paths';
	import { api } from '$lib/api';
	import { getErrorMessage } from '$lib/utils/errors';
	import { formatRelativeTimeAgo } from '$lib/utils/formatters';
	import { createRowClickHandler } from '$lib/utils/navigation';
	import { getServerColor } from '$lib/utils/server-colors';
	import * as Table from '$lib/components/ui/table';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { TracewayTableHeader } from '$lib/components/ui/traceway-table-header';
	import TableContainer from '$lib/components/traceway/table-container.svelte';
	import PageBadges from '$lib/components/traceway/page-badges.svelte';
	import ResourceRing from '$lib/components/traceway/resource-ring.svelte';
	import Sparkline from '$lib/components/dashboard/sparkline.svelte';
	import type { MetricTrendPoint } from '$lib/types/dashboard';
	import type { OncallPage } from '$lib/state/oncall.svelte';
	import {
		Activity,
		ArrowDown,
		ArrowRight,
		ArrowUp,
		Boxes,
		CircleCheck,
		Cloud,
		RefreshCw,
		Search,
		Server,
		TriangleAlert
	} from '@lucide/svelte';

	interface OrgServerRow {
		serverName: string;
		projectId: string;
		projectName: string;
		cpuPct: number | null;
		memoryPct: number | null;
		diskPct: number | null;
		networkRxBps: number | null;
		networkTxBps: number | null;
		lastReportedAt: string;
		trend: { timestamp: string; value: number }[];
		telemetrySource: 'otel' | 'sdk';
		dashboardId: number | null;
		hostName: string;
		hostId: string;
		hostArch: string;
		osType: string;
		osDescription: string;
		cloudProvider: string;
		cloudRegion: string;
		k8sClusterName: string;
		k8sNodeName: string;
	}

	interface OrgPageRow extends OncallPage {
		projectName: string;
	}

	type FleetFilter = 'all' | 'attention' | 'stale' | 'healthy';
	type GroupMode = 'project' | 'cluster' | 'none';
	type InstanceState = 'critical' | 'warning' | 'stale' | 'healthy';

	interface ServerGroup {
		key: string;
		label: string;
		subtitle: string;
		servers: OrgServerRow[];
	}

	let { organizationId }: { organizationId: number } = $props();

	const FRESH_MS = 3 * 60 * 1000;
	const REFRESH_MS = 60 * 1000;

	let servers = $state<OrgServerRow[]>([]);
	let serversLoading = $state(true);
	let serversError = $state('');
	let serversPartial = $state(false);
	let refreshedAt = $state<string | null>(null);
	let refreshing = $state(false);

	let totalGroups = $state(0);
	let issuesLoading = $state(true);
	let issuesError = $state('');

	let pages = $state<OrgPageRow[]>([]);
	let openPagesCount = $state(0);
	let downMonitorsCount = $state(0);
	let pagesLoading = $state(true);
	let pagesError = $state('');

	let searchQuery = $state('');
	let fleetFilter = $state<FleetFilter>('all');
	let projectFilter = $state('all');
	let groupMode = $state<GroupMode>('project');
	let groupPreferenceInitializedFor = $state<number | null>(null);

	let now = $state(Date.now());
	let loadGeneration = 0;
	let activeOrganizationId: number | null = null;

	async function loadServers(orgId: number, generation: number, background: boolean) {
		if (!background) serversLoading = true;
		serversError = '';
		try {
			const response = await api.get(`/organizations/${orgId}/overview/servers`);
			if (generation !== loadGeneration) return;
			servers = response.servers || [];
			serversPartial = response.partial === true;
			refreshedAt = response.refreshedAt || new Date().toISOString();
			now = Date.now();
		} catch (e) {
			if (generation !== loadGeneration) return;
			serversError = getErrorMessage(e) || 'Failed to load instance health';
		} finally {
			if (generation === loadGeneration) serversLoading = false;
		}
	}

	async function loadIssues(orgId: number, generation: number, background: boolean) {
		if (!background) issuesLoading = true;
		issuesError = '';
		try {
			const response = await api.get(`/organizations/${orgId}/overview/issues`);
			if (generation !== loadGeneration) return;
			totalGroups = response.totalGroups || 0;
		} catch (e) {
			if (generation !== loadGeneration) return;
			issuesError = getErrorMessage(e) || 'Failed to load issues';
		} finally {
			if (generation === loadGeneration) issuesLoading = false;
		}
	}

	async function loadPages(orgId: number, generation: number, background: boolean) {
		if (!background) pagesLoading = true;
		pagesError = '';
		try {
			const response = await api.get(`/organizations/${orgId}/overview/pages`);
			if (generation !== loadGeneration) return;
			pages = response.pages || [];
			openPagesCount = response.openPagesCount || 0;
			downMonitorsCount = response.downMonitorsCount || 0;
		} catch (e) {
			if (generation !== loadGeneration) return;
			pagesError = getErrorMessage(e) || 'Failed to load on-call pages';
		} finally {
			if (generation === loadGeneration) pagesLoading = false;
		}
	}

	async function refreshOverview(orgId: number, background: boolean) {
		const generation = ++loadGeneration;
		if (background) refreshing = true;
		await Promise.all([
			loadServers(orgId, generation, background),
			loadIssues(orgId, generation, background),
			loadPages(orgId, generation, background)
		]);
		if (generation === loadGeneration) refreshing = false;
	}

	$effect(() => {
		const orgId = organizationId;
		if (activeOrganizationId !== orgId) {
			activeOrganizationId = orgId;
			servers = [];
			serversPartial = false;
			refreshedAt = null;
			totalGroups = 0;
			pages = [];
			openPagesCount = 0;
			downMonitorsCount = 0;
			searchQuery = '';
			fleetFilter = 'all';
			projectFilter = 'all';
			groupMode = 'project';
			groupPreferenceInitializedFor = null;
		}
		void refreshOverview(orgId, false);
		const timer = window.setInterval(() => void refreshOverview(orgId, true), REFRESH_MS);
		return () => window.clearInterval(timer);
	});

	function isFresh(server: OrgServerRow): boolean {
		return now - new Date(server.lastReportedAt).getTime() <= FRESH_MS;
	}

	function atLeast(value: number | null, threshold: number): boolean {
		return value !== null && value >= threshold;
	}

	function instanceState(server: OrgServerRow): InstanceState {
		if (!isFresh(server)) return 'stale';
		if (
			atLeast(server.cpuPct, 95) ||
			atLeast(server.memoryPct, 95) ||
			atLeast(server.diskPct, 90)
		) {
			return 'critical';
		}
		if (
			atLeast(server.cpuPct, 80) ||
			atLeast(server.memoryPct, 85) ||
			atLeast(server.diskPct, 80)
		) {
			return 'warning';
		}
		return 'healthy';
	}

	const freshCount = $derived(servers.filter(isFresh).length);
	const staleCount = $derived(servers.length - freshCount);
	const attentionCount = $derived(
		servers.filter((server) => instanceState(server) !== 'healthy').length
	);
	const totalNetworkBps = $derived(
		servers.reduce(
			(total, server) => total + (server.networkRxBps ?? 0) + (server.networkTxBps ?? 0),
			0
		)
	);
	const allServerNames = $derived(servers.map((server) => server.serverName));
	const hasKubernetesMetadata = $derived(servers.some((server) => !!server.k8sClusterName));
	const projectOptions = $derived.by(() => {
		const projects = new Map<string, string>();
		for (const server of servers) projects.set(server.projectId, server.projectName);
		return [...projects.entries()]
			.map(([id, name]) => ({ id, name }))
			.sort((a, b) => a.name.localeCompare(b.name));
	});

	$effect(() => {
		if (serversLoading || groupPreferenceInitializedFor === organizationId) return;
		groupMode = hasKubernetesMetadata ? 'cluster' : 'project';
		groupPreferenceInitializedFor = organizationId;
	});

	function stateRank(state: InstanceState): number {
		return { critical: 0, stale: 1, warning: 2, healthy: 3 }[state];
	}

	const visibleServers = $derived.by(() => {
		const query = searchQuery.trim().toLowerCase();
		return servers
			.filter((server) => {
				const state = instanceState(server);
				if (projectFilter !== 'all' && server.projectId !== projectFilter) return false;
				if (fleetFilter === 'attention' && state === 'healthy') return false;
				if (fleetFilter === 'stale' && state !== 'stale') return false;
				if (fleetFilter === 'healthy' && state !== 'healthy') return false;
				if (!query) return true;
				return [
					server.serverName,
					server.projectName,
					server.hostName,
					server.hostId,
					server.osType,
					server.osDescription,
					server.hostArch,
					server.cloudProvider,
					server.cloudRegion,
					server.k8sClusterName,
					server.k8sNodeName
				]
					.join(' ')
					.toLowerCase()
					.includes(query);
			})
			.sort((a, b) => {
				const rank = stateRank(instanceState(a)) - stateRank(instanceState(b));
				if (rank !== 0) return rank;
				return a.serverName.localeCompare(b.serverName);
			});
	});

	const serverGroups = $derived.by((): ServerGroup[] => {
		if (groupMode === 'none') {
			return [{ key: 'all', label: 'All instances', subtitle: '', servers: visibleServers }];
		}

		const grouped = new Map<string, ServerGroup>();
		for (const server of visibleServers) {
			const cluster = server.k8sClusterName.trim();
			const key =
				groupMode === 'cluster'
					? cluster
						? `cluster:${cluster}`
						: 'cluster:ungrouped'
					: `project:${server.projectId}`;
			const label =
				groupMode === 'cluster' ? cluster || 'Not in a Kubernetes cluster' : server.projectName;
			let group = grouped.get(key);
			if (!group) {
				group = {
					key,
					label,
					subtitle: groupMode === 'cluster' && cluster ? 'Kubernetes cluster' : '',
					servers: []
				};
				grouped.set(key, group);
			}
			group.servers.push(server);
		}
		return [...grouped.values()].sort((a, b) => {
			if (a.key === 'cluster:ungrouped') return 1;
			if (b.key === 'cluster:ungrouped') return -1;
			return a.label.localeCompare(b.label);
		});
	});

	function toTrendPoints(trend: OrgServerRow['trend']): MetricTrendPoint[] {
		return (trend || []).map((point) => ({
			timestamp: new Date(point.timestamp),
			value: point.value
		}));
	}

	function formatByteRate(value: number | null): string {
		if (value === null) return '—';
		const units = ['B/s', 'KB/s', 'MB/s', 'GB/s'];
		let amount = Math.max(0, value);
		let unit = 0;
		while (amount >= 1000 && unit < units.length - 1) {
			amount /= 1000;
			unit++;
		}
		const precision = amount >= 100 ? 0 : amount >= 10 ? 1 : 2;
		return `${amount.toFixed(precision)} ${units[unit]}`;
	}

	function formatTotalNetwork(value: number): string {
		return servers.some((server) => server.networkRxBps !== null || server.networkTxBps !== null)
			? formatByteRate(value)
			: '—';
	}

	function stateLabel(state: InstanceState): string {
		return { critical: 'Critical', warning: 'Warning', stale: 'Stale', healthy: 'Healthy' }[state];
	}

	function stateClasses(state: InstanceState): string {
		return {
			critical: 'bg-rose-500/10 text-rose-600 ring-rose-500/20 dark:text-rose-400',
			warning: 'bg-amber-500/10 text-amber-700 ring-amber-500/20 dark:text-amber-400',
			stale: 'bg-muted text-muted-foreground ring-border',
			healthy: 'bg-emerald-500/10 text-emerald-700 ring-emerald-500/20 dark:text-emerald-400'
		}[state];
	}

	function instanceHref(server: OrgServerRow): string {
		const params = new URLSearchParams({
			projectId: server.projectId,
			server: server.serverName,
			preset: '30m'
		});
		if (server.dashboardId) params.set('dashboard', String(server.dashboardId));
		return `${resolve('/dashboards')}?${params.toString()}`;
	}

	function organizationTabHref(tab: string): string {
		return `${resolve('/organization')}?organizationId=${organizationId}&tab=${tab}`;
	}

	function osLabel(server: OrgServerRow): string {
		const type = server.osType.trim();
		if (!type) return '';
		return type.charAt(0).toUpperCase() + type.slice(1);
	}

	const fleetHeadline = $derived.by(() => {
		if (serversLoading && servers.length === 0) return 'Reading the fleet heartbeat';
		if (serversError && servers.length === 0) return 'Fleet signal unavailable';
		if (servers.length === 0) return 'No instance telemetry yet';
		if (attentionCount > 0) {
			return `${attentionCount} ${attentionCount === 1 ? 'instance needs' : 'instances need'} attention`;
		}
		return 'Fleet is reporting normally';
	});
</script>

<div class="space-y-6">
	<section
		class="overflow-hidden rounded-xl border bg-card shadow-xs"
		aria-labelledby="fleet-pulse"
	>
		<div
			class="flex flex-col gap-4 px-5 py-5 sm:flex-row sm:items-start sm:justify-between sm:px-6"
		>
			<div class="flex min-w-0 items-start gap-3.5">
				<div
					class="mt-0.5 grid size-10 shrink-0 place-items-center rounded-xl {attentionCount > 0
						? 'bg-amber-500/10 text-amber-600 dark:text-amber-400'
						: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'}"
				>
					{#if attentionCount > 0}
						<TriangleAlert class="size-5" />
					{:else}
						<Activity class="size-5" />
					{/if}
				</div>
				<div class="min-w-0">
					<div class="text-xs font-semibold tracking-[0.16em] text-muted-foreground uppercase">
						Operational pulse
					</div>
					<h2 id="fleet-pulse" class="mt-1 text-xl font-semibold tracking-tight">
						{fleetHeadline}
					</h2>
					<p class="mt-1 text-sm text-muted-foreground">
						{#if refreshedAt}
							Updated {formatRelativeTimeAgo(refreshedAt)} · refreshes every minute
						{:else}
							Live resource health across every project
						{/if}
					</p>
				</div>
			</div>
			<Button
				variant="outline"
				size="sm"
				disabled={refreshing}
				onclick={() => void refreshOverview(organizationId, true)}
			>
				<RefreshCw class="size-4 {refreshing ? 'animate-spin' : ''}" />
				Refresh
			</Button>
		</div>

		<div class="grid grid-cols-2 border-t sm:grid-cols-3 xl:grid-cols-6">
			<div class="border-r border-b px-5 py-4 xl:border-b-0">
				<div class="text-xs text-muted-foreground">Reporting now</div>
				<div
					class="mt-1 font-mono text-xl font-semibold text-emerald-600 tabular-nums dark:text-emerald-400"
				>
					{serversLoading && servers.length === 0 ? '—' : freshCount}
				</div>
			</div>
			<div class="border-r border-b px-5 py-4 sm:border-r xl:border-b-0">
				<div class="text-xs text-muted-foreground">Needs attention</div>
				<div
					class="mt-1 font-mono text-xl font-semibold tabular-nums {attentionCount > 0
						? 'text-amber-600 dark:text-amber-400'
						: ''}"
				>
					{serversLoading && servers.length === 0 ? '—' : attentionCount}
				</div>
			</div>
			<div class="border-r border-b px-5 py-4 sm:border-r-0 xl:border-r xl:border-b-0">
				<div class="text-xs text-muted-foreground">Stale signals</div>
				<div class="mt-1 font-mono text-xl font-semibold tabular-nums">
					{serversLoading && servers.length === 0 ? '—' : staleCount}
				</div>
			</div>
			<div class="border-r border-b px-5 py-4 sm:border-b-0 xl:border-r">
				<div class="text-xs text-muted-foreground">Network I/O</div>
				<div class="mt-1 font-mono text-xl font-semibold tabular-nums">
					{formatTotalNetwork(totalNetworkBps)}
				</div>
			</div>
			<a
				href={organizationTabHref('issues')}
				class="group border-r px-5 py-4 transition-colors hover:bg-muted/40 focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none focus-visible:ring-inset"
			>
				<div class="flex items-center justify-between text-xs text-muted-foreground">
					Issues · 24h
					<ArrowRight class="size-3.5 opacity-0 transition-opacity group-hover:opacity-100" />
				</div>
				<div class="mt-1 font-mono text-xl font-semibold tabular-nums">
					{issuesLoading || issuesError ? '—' : totalGroups}
				</div>
			</a>
			<a
				href={organizationTabHref('monitors')}
				class="group px-5 py-4 transition-colors hover:bg-muted/40 focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none focus-visible:ring-inset"
			>
				<div class="flex items-center justify-between text-xs text-muted-foreground">
					Active response
					<ArrowRight class="size-3.5 opacity-0 transition-opacity group-hover:opacity-100" />
				</div>
				<div
					class="mt-1 font-mono text-xl font-semibold tabular-nums {openPagesCount +
						downMonitorsCount >
					0
						? 'text-rose-600 dark:text-rose-400'
						: ''}"
				>
					{pagesLoading || pagesError ? '—' : openPagesCount + downMonitorsCount}
				</div>
			</a>
		</div>
	</section>

	<section class="space-y-3" aria-labelledby="instances-heading">
		<div class="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
			<div>
				<div class="flex items-center gap-2">
					<h2 id="instances-heading" class="text-lg font-semibold">Instances</h2>
					{#if !serversLoading}
						<span
							class="rounded-full bg-muted px-2 py-0.5 font-mono text-xs text-muted-foreground tabular-nums"
							>{servers.length}</span
						>
					{/if}
				</div>
				<p class="mt-0.5 text-sm text-muted-foreground">
					CPU, memory, storage, and network from the latest 30 minutes.
				</p>
			</div>

			{#if servers.length > 0}
				<div class="flex flex-col gap-2 sm:flex-row sm:items-center">
					<div class="relative min-w-0 sm:w-56">
						<Search
							class="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground"
						/>
						<Input bind:value={searchQuery} placeholder="Find an instance" class="h-9 pl-9" />
					</div>
					<select
						bind:value={fleetFilter}
						aria-label="Filter instances by health"
						class="h-9 rounded-md border border-input bg-background px-3 text-sm shadow-xs outline-none focus-visible:ring-2 focus-visible:ring-ring"
					>
						<option value="all">All health states</option>
						<option value="attention">Needs attention</option>
						<option value="stale">Stale</option>
						<option value="healthy">Healthy</option>
					</select>
					{#if projectOptions.length > 1}
						<select
							bind:value={projectFilter}
							aria-label="Filter instances by project"
							class="h-9 max-w-48 rounded-md border border-input bg-background px-3 text-sm shadow-xs outline-none focus-visible:ring-2 focus-visible:ring-ring"
						>
							<option value="all">All projects</option>
							{#each projectOptions as project (project.id)}
								<option value={project.id}>{project.name}</option>
							{/each}
						</select>
					{/if}
					<select
						bind:value={groupMode}
						aria-label="Group instances"
						class="h-9 rounded-md border border-input bg-background px-3 text-sm shadow-xs outline-none focus-visible:ring-2 focus-visible:ring-ring"
					>
						<option value="project">Group by project</option>
						{#if hasKubernetesMetadata}
							<option value="cluster">Group by Kubernetes cluster</option>
						{/if}
						<option value="none">No grouping</option>
					</select>
				</div>
			{/if}
		</div>

		{#if serversPartial || (serversError && servers.length > 0)}
			<div
				class="flex items-center gap-2 rounded-lg border border-amber-500/25 bg-amber-500/5 px-4 py-2.5 text-sm text-amber-700 dark:text-amber-300"
			>
				<TriangleAlert class="size-4 shrink-0" />
				{serversPartial
					? 'Some projects could not be read. The instances shown are still current.'
					: 'The last refresh failed. Showing the most recent snapshot.'}
			</div>
		{/if}

		{#if serversLoading && servers.length === 0}
			<div class="flex h-48 items-center justify-center rounded-xl border">
				<LoadingCircle size="lg" />
			</div>
		{:else if serversError && servers.length === 0}
			<div
				class="flex flex-col items-center justify-center gap-3 rounded-xl border py-12 text-center"
			>
				<TriangleAlert class="size-6 text-rose-500" />
				<div>
					<p class="font-medium">Instance health could not be loaded</p>
					<p class="mt-1 text-sm text-muted-foreground">{serversError}</p>
				</div>
				<Button
					variant="outline"
					size="sm"
					onclick={() => void refreshOverview(organizationId, false)}>Retry</Button
				>
			</div>
		{:else if servers.length === 0}
			<div
				class="flex flex-col items-center justify-center rounded-xl border border-dashed py-12 text-center"
			>
				<div class="grid size-11 place-items-center rounded-xl bg-muted text-muted-foreground">
					<Server class="size-5" />
				</div>
				<p class="mt-3 font-medium">No instance telemetry in the last 30 minutes</p>
				<p class="mt-1 max-w-md text-sm text-muted-foreground">
					Install the Traceway OTel Agent on a host, or send server metrics with a unique service
					name.
				</p>
			</div>
		{:else if visibleServers.length === 0}
			<div class="rounded-xl border border-dashed py-10 text-center">
				<p class="font-medium">No instances match these filters</p>
				<button
					class="mt-2 text-sm font-medium text-primary hover:underline"
					onclick={() => {
						searchQuery = '';
						fleetFilter = 'all';
						projectFilter = 'all';
					}}
				>
					Clear filters
				</button>
			</div>
		{:else}
			<div class="overflow-hidden rounded-xl border bg-card shadow-xs">
				<div
					class="hidden grid-cols-[minmax(250px,1.7fr)_104px_104px_104px_minmax(155px,0.9fr)_125px_28px] items-center border-b bg-muted/30 px-4 py-2.5 text-[11px] font-semibold tracking-wider text-muted-foreground uppercase lg:grid"
				>
					<div>Instance</div>
					<div class="text-center">CPU</div>
					<div class="text-center">Memory</div>
					<div class="text-center">Disk</div>
					<div>Network</div>
					<div>Last signal</div>
					<div></div>
				</div>

				{#each serverGroups as group (group.key)}
					<div
						class="flex items-center justify-between border-b bg-muted/20 px-4 py-2.5 last:border-b-0"
					>
						<div class="flex min-w-0 items-center gap-2">
							{#if groupMode === 'cluster' && group.key !== 'cluster:ungrouped'}
								<Boxes class="size-4 shrink-0 text-muted-foreground" />
							{:else}
								<Cloud class="size-4 shrink-0 text-muted-foreground" />
							{/if}
							<span class="truncate text-sm font-semibold">{group.label}</span>
							{#if group.subtitle}
								<span class="hidden text-xs text-muted-foreground sm:inline">{group.subtitle}</span>
							{/if}
						</div>
						<span class="font-mono text-xs text-muted-foreground tabular-nums"
							>{group.servers.length}</span
						>
					</div>

					{#each group.servers as server (server.projectId + server.serverName)}
						{@const state = instanceState(server)}
						<a
							href={instanceHref(server)}
							class="group/instance grid gap-4 border-b px-4 py-4 transition-colors last:border-b-0 hover:bg-muted/35 focus-visible:bg-muted/35 focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none focus-visible:ring-inset lg:grid-cols-[minmax(250px,1.7fr)_104px_104px_104px_minmax(155px,0.9fr)_125px_28px] lg:items-center lg:gap-0 lg:py-3"
						>
							<div class="flex min-w-0 items-center gap-3">
								<div
									class="relative grid size-9 shrink-0 place-items-center rounded-lg bg-muted text-muted-foreground"
								>
									{#if server.k8sClusterName}
										<Boxes class="size-4" />
									{:else}
										<Server class="size-4" />
									{/if}
									<span
										class="absolute -right-0.5 -bottom-0.5 size-2.5 rounded-full border-2 border-card {state ===
										'healthy'
											? 'bg-emerald-500'
											: state === 'warning'
												? 'bg-amber-500'
												: state === 'critical'
													? 'bg-rose-500'
													: 'bg-muted-foreground'}"
									></span>
								</div>
								<div class="min-w-0 flex-1">
									<div class="flex min-w-0 items-center gap-2">
										<span class="truncate font-mono text-sm font-semibold">{server.serverName}</span
										>
										<span
											class="shrink-0 rounded-full px-2 py-0.5 text-[10px] font-semibold ring-1 ring-inset {stateClasses(
												state
											)}">{stateLabel(state)}</span
										>
									</div>
									<div
										class="mt-1 flex min-w-0 flex-wrap items-center gap-x-2 gap-y-0.5 text-xs text-muted-foreground"
									>
										{#if groupMode !== 'project'}<span class="truncate">{server.projectName}</span
											>{/if}
										{#if server.hostName && server.hostName !== server.serverName}<span
												class="font-mono">{server.hostName}</span
											>{/if}
										{#if osLabel(server)}<span title={server.osDescription || undefined}
												>{osLabel(server)}{#if server.hostArch}
													· {server.hostArch}{/if}</span
											>{/if}
										{#if server.cloudRegion}<span>{server.cloudRegion}</span>{/if}
										{#if server.k8sNodeName}<span>node {server.k8sNodeName}</span>{/if}
										<span>{server.telemetrySource === 'otel' ? 'OTel Agent' : 'SDK metrics'}</span>
									</div>
								</div>
							</div>

							<div class="grid grid-cols-3 gap-3 lg:contents">
								<div class="flex flex-col items-center gap-1">
									<span
										class="text-[10px] font-semibold tracking-wider text-muted-foreground uppercase lg:hidden"
										>CPU</span
									>
									<ResourceRing label="CPU" value={server.cpuPct} size="sm" />
									{#if server.trend.length > 1}
										<div class="hidden h-3 w-14 lg:block">
											<Sparkline
												data={toTrendPoints(server.trend)}
												color={getServerColor(server.serverName, allServerNames)}
												height={12}
											/>
										</div>
									{/if}
								</div>
								<div class="flex flex-col items-center gap-1">
									<span
										class="text-[10px] font-semibold tracking-wider text-muted-foreground uppercase lg:hidden"
										>Memory</span
									>
									<ResourceRing label="Memory" value={server.memoryPct} warningAt={85} size="sm" />
								</div>
								<div class="flex flex-col items-center gap-1">
									<span
										class="text-[10px] font-semibold tracking-wider text-muted-foreground uppercase lg:hidden"
										>Disk</span
									>
									<ResourceRing
										label="Disk"
										value={server.diskPct}
										warningAt={80}
										criticalAt={90}
										size="sm"
									/>
								</div>
							</div>

							<div
								class="grid grid-cols-2 gap-2 border-t pt-3 font-mono text-xs tabular-nums lg:block lg:border-0 lg:pt-0"
							>
								<div class="flex items-center gap-1.5 text-muted-foreground">
									<ArrowDown class="size-3.5 text-cyan-500" />
									<span>{formatByteRate(server.networkRxBps)}</span>
								</div>
								<div class="mt-0 flex items-center gap-1.5 text-muted-foreground lg:mt-1">
									<ArrowUp class="size-3.5 text-violet-500" />
									<span>{formatByteRate(server.networkTxBps)}</span>
								</div>
							</div>

							<div
								class="flex items-center justify-between gap-2 text-xs text-muted-foreground lg:block"
							>
								<span class="lg:hidden">Last signal</span>
								<span class="font-mono tabular-nums"
									>{formatRelativeTimeAgo(server.lastReportedAt)}</span
								>
							</div>
							<ArrowRight
								class="hidden size-4 text-muted-foreground transition-transform group-hover/instance:translate-x-0.5 group-hover/instance:text-foreground lg:block"
							/>
						</a>
					{/each}
				{/each}
			</div>
		{/if}
	</section>

	<section class="space-y-3" aria-labelledby="pages-heading">
		<div class="flex items-center justify-between">
			<div>
				<h2 id="pages-heading" class="text-lg font-semibold">Active on-call response</h2>
				<p class="mt-0.5 text-sm text-muted-foreground">
					Pages that still need acknowledgement or resolution.
				</p>
			</div>
			{#if openPagesCount > 0}
				<span
					class="rounded-full bg-rose-500/10 px-2.5 py-1 font-mono text-xs font-semibold text-rose-600 tabular-nums dark:text-rose-400"
					>{openPagesCount} open</span
				>
			{/if}
		</div>

		{#if pagesLoading && pages.length === 0}
			<div class="flex h-28 items-center justify-center rounded-xl border">
				<LoadingCircle size="lg" />
			</div>
		{:else if pagesError && pages.length === 0}
			<div class="rounded-xl border py-8 text-center text-sm text-rose-500">{pagesError}</div>
		{:else if pages.length === 0}
			<div class="flex items-center gap-3 rounded-xl border bg-card px-5 py-4">
				<div
					class="grid size-9 shrink-0 place-items-center rounded-full bg-emerald-500/10 text-emerald-600 dark:text-emerald-400"
				>
					<CircleCheck class="size-5" />
				</div>
				<div>
					<p class="text-sm font-medium">No active pages</p>
					<p class="text-xs text-muted-foreground">Nobody is being paged right now.</p>
				</div>
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
								<Table.Cell><PageBadges status={oncallPage.status} /></Table.Cell>
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
