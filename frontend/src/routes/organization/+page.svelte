<script lang="ts">
	import { api } from '$lib/api';
	import { organizationContext } from '$lib/state/organization-context.svelte';
	import { getErrorMessage } from '$lib/utils/errors';
	import { formatRelativeTimeAgo } from '$lib/utils/formatters';
	import { resolveHref } from '$lib/utils/links';
	import { getServerColor } from '$lib/utils/server-colors';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import * as Select from '$lib/components/ui/select';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import PageHeader from '$lib/components/traceway/page-header.svelte';
	import ResourceRing from '$lib/components/traceway/resource-ring.svelte';
	import Sparkline from '$lib/components/dashboard/sparkline.svelte';
	import type { MetricTrendPoint } from '$lib/types/dashboard';
	import CountPill from './count-pill.svelte';
	import PartialNotice from './partial-notice.svelte';
	import {
		ArrowDown,
		ArrowRight,
		ArrowUp,
		Boxes,
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

	type ServerState = 'critical' | 'warning' | 'stale' | 'healthy';
	type StateFilter = 'all' | ServerState;
	type GroupMode = 'project' | 'cluster' | 'none';
	type RefreshMode = 'initial' | 'background' | 'manual';

	const GROUP_LABELS: Record<GroupMode, string> = {
		project: 'Group by project',
		cluster: 'Group by Kubernetes cluster',
		none: 'No grouping'
	};

	interface ServerGroup {
		key: string;
		label: string;
		subtitle: string;
		servers: OrgServerRow[];
	}

	const FRESH_MS = 3 * 60 * 1000;
	const REFRESH_MS = 60 * 1000;

	const STATE_ORDER: ServerState[] = ['critical', 'stale', 'warning', 'healthy'];
	const STATE_LABELS: Record<ServerState, string> = {
		critical: 'critical',
		stale: 'stale',
		warning: 'warning',
		healthy: 'healthy'
	};
	const STATE_DOT: Record<ServerState, string> = {
		critical: 'bg-rose-500',
		stale: 'bg-muted-foreground/50',
		warning: 'bg-amber-500',
		healthy: 'bg-emerald-500'
	};
	const STATE_BADGE: Record<ServerState, string> = {
		critical: 'bg-rose-500/10 text-rose-600 ring-rose-500/20 dark:text-rose-400',
		stale: 'bg-muted text-muted-foreground ring-border',
		warning: 'bg-amber-500/10 text-amber-700 ring-amber-500/20 dark:text-amber-400',
		healthy: 'bg-emerald-500/10 text-emerald-700 ring-emerald-500/20 dark:text-emerald-400'
	};

	let servers = $state<OrgServerRow[]>([]);
	let loading = $state(true);
	let error = $state('');
	let partial = $state(false);
	let refreshedAt = $state<string | null>(null);
	let refreshing = $state(false);

	let searchQuery = $state('');
	let stateFilter = $state<StateFilter>('all');
	let projectFilter = $state('all');
	let groupMode = $state<GroupMode>('project');
	let groupPreferenceInitializedFor = $state<number | null>(null);

	let now = $state(Date.now());
	let loadGeneration = 0;
	let activeOrganizationId: number | null = null;

	async function loadServers(organizationId: number, mode: RefreshMode) {
		const generation = ++loadGeneration;
		if (mode === 'initial') loading = true;
		if (mode === 'manual') refreshing = true;
		error = '';
		try {
			const response = await api.get(`/organizations/${organizationId}/overview/servers`);
			if (generation !== loadGeneration) return;
			servers = response.servers || [];
			partial = response.partial === true;
			now = Date.now();
			refreshedAt = new Date(now).toISOString();
		} catch (e) {
			if (generation !== loadGeneration) return;
			error = getErrorMessage(e) || 'Failed to load servers';
		} finally {
			if (generation === loadGeneration) {
				loading = false;
				refreshing = false;
			}
		}
	}

	function refresh(mode: RefreshMode) {
		const organizationId = organizationContext.organizationId;
		if (organizationId !== null) void loadServers(organizationId, mode);
	}

	$effect(() => {
		const organizationId = organizationContext.organizationId;
		if (organizationId === null) return;
		if (activeOrganizationId !== organizationId) {
			activeOrganizationId = organizationId;
			servers = [];
			partial = false;
			refreshedAt = null;
			searchQuery = '';
			stateFilter = 'all';
			projectFilter = 'all';
			groupMode = 'project';
			groupPreferenceInitializedFor = null;
		}
		void loadServers(organizationId, 'initial');
		const timer = window.setInterval(
			() => void loadServers(organizationId, 'background'),
			REFRESH_MS
		);
		return () => window.clearInterval(timer);
	});

	function isFresh(server: OrgServerRow): boolean {
		return now - new Date(server.lastReportedAt).getTime() <= FRESH_MS;
	}

	function atLeast(value: number | null, threshold: number): boolean {
		return value !== null && value >= threshold;
	}

	function serverState(server: OrgServerRow): ServerState {
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

	function stateRank(state: ServerState): number {
		return STATE_ORDER.indexOf(state);
	}

	const stateCounts = $derived.by(() => {
		const counts: Record<ServerState, number> = { critical: 0, stale: 0, warning: 0, healthy: 0 };
		for (const server of servers) counts[serverState(server)]++;
		return counts;
	});
	const attentionCount = $derived(servers.length - stateCounts.healthy);
	const allServerNames = $derived(servers.map((server) => server.serverName));
	const hasKubernetesMetadata = $derived(servers.some((server) => !!server.k8sClusterName));
	const projectOptions = $derived.by(() => {
		const byProject = new Map(
			servers.map((server) => [
				server.projectId,
				{ id: server.projectId, name: server.projectName }
			])
		);
		return [...byProject.values()].sort((a, b) => a.name.localeCompare(b.name));
	});

	$effect(() => {
		if (loading || groupPreferenceInitializedFor === organizationContext.organizationId) return;
		groupMode = hasKubernetesMetadata ? 'cluster' : 'project';
		groupPreferenceInitializedFor = organizationContext.organizationId;
	});

	const visibleServers = $derived.by(() => {
		const query = searchQuery.trim().toLowerCase();
		return servers
			.filter((server) => {
				const state = serverState(server);
				if (projectFilter !== 'all' && server.projectId !== projectFilter) return false;
				if (stateFilter !== 'all' && state !== stateFilter) return false;
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
				const rank = stateRank(serverState(a)) - stateRank(serverState(b));
				if (rank !== 0) return rank;
				return a.serverName.localeCompare(b.serverName);
			});
	});

	const serverGroups = $derived.by((): ServerGroup[] => {
		if (groupMode === 'none') {
			return [{ key: 'all', label: 'All servers', subtitle: '', servers: visibleServers }];
		}

		// eslint-disable-next-line svelte/prefer-svelte-reactivity
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

	const filtersActive = $derived(
		searchQuery.trim() !== '' || stateFilter !== 'all' || projectFilter !== 'all'
	);

	const headline = $derived.by(() => {
		if (attentionCount === 0) {
			return servers.length === 1 ? 'The only server is healthy' : 'All servers are healthy';
		}
		const verb = attentionCount === 1 ? 'needs' : 'need';
		return `${attentionCount} of ${servers.length} servers ${verb} attention`;
	});

	function toggleStateFilter(state: ServerState) {
		stateFilter = stateFilter === state ? 'all' : state;
	}

	function clearFilters() {
		searchQuery = '';
		stateFilter = 'all';
		projectFilter = 'all';
	}

	const projectFilterLabel = $derived(
		projectOptions.find((project) => project.id === projectFilter)?.name ?? 'All projects'
	);

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

	function serverHref(server: OrgServerRow): string {
		const params = new URLSearchParams({
			projectId: server.projectId,
			server: server.serverName,
			preset: '30m',
			...(server.dashboardId ? { dashboard: String(server.dashboardId) } : {})
		});
		return `/dashboards?${params.toString()}`;
	}

	function osLabel(server: OrgServerRow): string {
		const type = server.osType.trim();
		if (!type) return '';
		return type.charAt(0).toUpperCase() + type.slice(1);
	}
</script>

<div class="space-y-5">
	<PageHeader
		title="Servers"
		description="CPU, memory, disk, and network for every server that reported in the last 30 minutes, across all projects."
	>
		{#snippet trailing()}
			{#if !loading}
				<CountPill count={servers.length} />
			{/if}
		{/snippet}
		{#snippet actions()}
			<div class="flex items-center gap-3">
				{#if refreshedAt}
					<span class="text-xs text-muted-foreground">
						Updated {formatRelativeTimeAgo(refreshedAt)}
					</span>
				{/if}
				<Button variant="outline" size="sm" disabled={refreshing} onclick={() => refresh('manual')}>
					<RefreshCw class="size-4 {refreshing ? 'animate-spin' : ''}" />
					Refresh
				</Button>
			</div>
		{/snippet}
	</PageHeader>

	{#if servers.length > 0}
		<section aria-label="Fleet health" class="space-y-2.5">
			<div class="flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
				<p class="flex items-center gap-2 text-sm font-medium">
					<span
						class="size-2 shrink-0 rounded-full {attentionCount > 0
							? 'bg-amber-500'
							: 'bg-emerald-500'}"
					></span>
					{headline}
				</p>
				<div
					class="flex flex-wrap gap-1.5"
					role="group"
					aria-label="Filter servers by health state"
				>
					{#each STATE_ORDER as state (state)}
						{@const count = stateCounts[state]}
						<button
							type="button"
							aria-pressed={stateFilter === state}
							disabled={count === 0 && stateFilter !== state}
							onclick={() => toggleStateFilter(state)}
							class="inline-flex items-center gap-1.5 rounded-full border bg-background px-2.5 py-1 text-xs font-medium tabular-nums transition-colors hover:bg-muted/60 focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none disabled:opacity-40 disabled:hover:bg-background aria-pressed:border-foreground aria-pressed:bg-foreground aria-pressed:text-background aria-pressed:hover:bg-foreground"
						>
							<span class="size-2 rounded-full {STATE_DOT[state]}"></span>
							{count}
							{STATE_LABELS[state]}
						</button>
					{/each}
				</div>
			</div>
			<div class="flex h-2 w-full gap-0.5 overflow-hidden rounded-full bg-muted" aria-hidden="true">
				{#each STATE_ORDER as state (state)}
					{#if stateCounts[state] > 0}
						<div
							class="h-full transition-[flex] duration-300 motion-reduce:transition-none {STATE_DOT[
								state
							]}"
							style="flex: {stateCounts[state]} 1 0%"
						></div>
					{/if}
				{/each}
			</div>
		</section>

		<div class="flex flex-col gap-2 sm:flex-row sm:items-center">
			<div class="relative min-w-0 sm:w-64">
				<Search
					class="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground"
				/>
				<Input bind:value={searchQuery} placeholder="Find a server" class="h-9 pl-9" />
			</div>
			{#if projectOptions.length > 1}
				<Select.Root type="single" bind:value={projectFilter}>
					<Select.Trigger class="h-9 w-full sm:w-48" aria-label="Filter servers by project">
						<span class="truncate">{projectFilterLabel}</span>
					</Select.Trigger>
					<Select.Content>
						<Select.Item value="all" label="All projects">All projects</Select.Item>
						{#each projectOptions as project (project.id)}
							<Select.Item value={project.id} label={project.name}>{project.name}</Select.Item>
						{/each}
					</Select.Content>
				</Select.Root>
			{/if}
			<Select.Root
				type="single"
				value={groupMode}
				onValueChange={(value) => (groupMode = value as GroupMode)}
			>
				<Select.Trigger class="h-9 w-full sm:w-auto" aria-label="Group servers">
					{GROUP_LABELS[groupMode]}
				</Select.Trigger>
				<Select.Content>
					<Select.Item value="project" label={GROUP_LABELS.project}
						>{GROUP_LABELS.project}</Select.Item
					>
					{#if hasKubernetesMetadata}
						<Select.Item value="cluster" label={GROUP_LABELS.cluster}
							>{GROUP_LABELS.cluster}</Select.Item
						>
					{/if}
					<Select.Item value="none" label={GROUP_LABELS.none}>{GROUP_LABELS.none}</Select.Item>
				</Select.Content>
			</Select.Root>
			{#if filtersActive}
				<span class="text-sm text-muted-foreground tabular-nums sm:ml-auto">
					Showing {visibleServers.length} of {servers.length}
				</span>
			{/if}
		</div>
	{/if}

	{#if partial || (error && servers.length > 0)}
		<PartialNotice
			message={partial
				? 'Some projects could not be read. The servers shown are still current.'
				: 'The last refresh failed. Showing the most recent snapshot.'}
		/>
	{/if}

	{#if loading && servers.length === 0}
		<div class="flex h-48 items-center justify-center rounded-xl border">
			<LoadingCircle size="lg" />
		</div>
	{:else if error && servers.length === 0}
		<div
			class="flex flex-col items-center justify-center gap-3 rounded-xl border py-12 text-center"
		>
			<TriangleAlert class="size-6 text-rose-500" />
			<div>
				<p class="font-medium">Servers could not be loaded</p>
				<p class="mt-1 text-sm text-muted-foreground">{error}</p>
			</div>
			<Button variant="outline" size="sm" onclick={() => refresh('initial')}>Retry</Button>
		</div>
	{:else if servers.length === 0}
		<div
			class="flex flex-col items-center justify-center rounded-xl border border-dashed py-12 text-center"
		>
			<div class="grid size-11 place-items-center rounded-xl bg-muted text-muted-foreground">
				<Server class="size-5" />
			</div>
			<p class="mt-3 font-medium">No servers reported in the last 30 minutes</p>
			<p class="mt-1 max-w-md text-sm text-muted-foreground">
				Install the Traceway OTel Agent on a host, or send server metrics with a unique service
				name.
			</p>
		</div>
	{:else if visibleServers.length === 0}
		<div class="rounded-xl border border-dashed py-10 text-center">
			<p class="font-medium">No servers match these filters</p>
			<button class="mt-2 text-sm font-medium text-primary hover:underline" onclick={clearFilters}>
				Clear filters
			</button>
		</div>
	{:else}
		<div class="overflow-hidden rounded-xl border bg-card shadow-xs">
			<div
				class="hidden grid-cols-[minmax(250px,1.7fr)_104px_104px_104px_minmax(155px,0.9fr)_125px_28px] items-center border-b bg-muted/30 px-4 py-2.5 text-[11px] font-semibold tracking-wider text-muted-foreground uppercase lg:grid"
			>
				<div>Server</div>
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
					{@const state = serverState(server)}
					<a
						{...{ href: resolveHref(serverHref(server)) }}
						class="group/server grid gap-4 border-b px-4 py-4 transition-colors last:border-b-0 hover:bg-muted/35 focus-visible:bg-muted/35 focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none focus-visible:ring-inset lg:grid-cols-[minmax(250px,1.7fr)_104px_104px_104px_minmax(155px,0.9fr)_125px_28px] lg:items-center lg:gap-0 lg:py-3"
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
									class="absolute -right-0.5 -bottom-0.5 size-2.5 rounded-full border-2 border-card {STATE_DOT[
										state
									]}"
								></span>
							</div>
							<div class="min-w-0 flex-1">
								<div class="flex min-w-0 items-center gap-2">
									<span class="truncate font-mono text-sm font-semibold">{server.serverName}</span>
									<span
										class="shrink-0 rounded-full px-2 py-0.5 text-[10px] font-semibold capitalize ring-1 ring-inset {STATE_BADGE[
											state
										]}">{STATE_LABELS[state]}</span
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
											>{osLabel(server)}{server.hostArch ? ` · ${server.hostArch}` : ''}</span
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
							class="hidden size-4 text-muted-foreground transition-transform group-hover/server:translate-x-0.5 group-hover/server:text-foreground lg:block"
						/>
					</a>
				{/each}
			{/each}
		</div>
	{/if}
</div>
