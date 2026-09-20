<script lang="ts">
	import { untrack } from 'svelte';
	import { SvelteMap } from 'svelte/reactivity';
	import { api } from '$lib/api';
	import * as Card from '$lib/components/ui/card';
	import * as Table from '$lib/components/ui/table';
	import * as Tabs from '$lib/components/ui/tabs';
	import { Button } from '$lib/components/ui/button';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { SeverityBadge } from '$lib/components/ui/severity-badge';
	import ExpandedLogRow from './expanded-log-row.svelte';
	import LogMessage from './log-message.svelte';
	import { formatDateTime } from '$lib/utils/formatters';
	import { getTimezone } from '$lib/state/timezone.svelte';
	import { underlineTabTriggerClass, underlineTabListClass } from '$lib/utils/tabs';
	import { cn } from '$lib/utils';
	import type { Span } from '$lib/types/spans';

	type LogRecord = {
		id: string;
		projectId: string;
		timestamp: string;
		traceId: string;
		spanId: string;
		severityText: string;
		severityNumber: number;
		serviceName: string;
		body: string;
		resourceAttributes: Record<string, string> | null;
		scopeName: string;
		scopeVersion: string;
		scopeAttributes: Record<string, string> | null;
		logAttributes: Record<string, string> | null;
	};

	let {
		projectId,
		traceId,
		distributedTraceId,
		spans,
		rootSpan,
		traceRecordedAt
	}: {
		projectId: string;
		traceId: string;
		distributedTraceId?: string | null;
		spans: Span[];
		rootSpan?: Pick<Span, 'spanId' | 'name'>;
		traceRecordedAt: string;
	} = $props();

	const timezone = $derived(getTimezone());
	const relatedTraceId = $derived(distributedTraceId === undefined ? traceId : distributedTraceId);
	let generation = 0;

	let activeTab = $state<'this-trace' | 'all-distributed'>('this-trace');

	const tabTriggerClass = cn(underlineTabTriggerClass, 'mb-0 pb-2');

	let logs = $state<LogRecord[]>([]);
	let loading = $state(true);
	let error = $state('');
	let expandedId = $state<string | null>(null);
	let logsPage = $state(1);
	let totalLogs = $state(0);

	let distributedLogs = $state<LogRecord[]>([]);
	let distributedLoading = $state(false);
	let distributedLoaded = $state(false);
	let distributedError = $state('');
	let distributedPage = $state(1);
	let totalDistributedLogs = $state(0);

	const spanNameByHex = $derived.by(() => {
		const m = new SvelteMap<string, string>();
		if (rootSpan) m.set(rootSpan.spanId.toLowerCase(), rootSpan.name);
		for (const s of spans) {
			m.set(s.spanId.toLowerCase(), s.name);
		}
		return m;
	});

	function resolveSpanName(log: LogRecord): string | null {
		if (!log.spanId) return null;
		return spanNameByHex.get(log.spanId.toLowerCase()) ?? null;
	}

	function timeWindow(): { fromDate: string; toDate: string } {
		const t = new Date(traceRecordedAt).getTime();
		const hour = 60 * 60 * 1000;
		return {
			fromDate: new Date(t - hour).toISOString(),
			toDate: new Date(t + hour).toISOString()
		};
	}

	async function loadTraceLogs(page = 1) {
		const requestGeneration = generation;
		loading = true;
		error = '';
		try {
			const { fromDate, toDate } = timeWindow();
			const response = (await api.post(
				'/logs',
				{
					fromDate,
					toDate,
					orderBy: 'timestamp',
					sortDirection: 'asc',
					pagination: { page, pageSize: 100 },
					traceId
				},
				{ projectId: projectId || undefined }
			)) as { data: LogRecord[]; pagination?: { total: number } };
			if (requestGeneration === generation) {
				logs = response.data || [];
				logsPage = page;
				totalLogs = response.pagination?.total ?? logs.length;
				expandedId = null;
			}
		} catch (e: unknown) {
			if (requestGeneration !== generation) return;
			const err = e as { message?: string };
			error = err.message || 'Failed to load logs';
		} finally {
			if (requestGeneration === generation) loading = false;
		}
	}

	async function loadDistributedLogs(page = 1) {
		if (!relatedTraceId) return;
		const requestGeneration = generation;
		distributedLoading = true;
		distributedError = '';
		try {
			const { fromDate, toDate } = timeWindow();
			const response = (await api.post(
				'/logs',
				{
					fromDate,
					toDate,
					orderBy: 'timestamp',
					sortDirection: 'asc',
					pagination: { page, pageSize: 100 },
					distributedTraceId: relatedTraceId
				},
				{ projectId: projectId || undefined }
			)) as { data: LogRecord[]; pagination?: { total: number } };
			if (requestGeneration === generation) {
				distributedLogs = response.data || [];
				distributedLoaded = true;
				distributedPage = page;
				totalDistributedLogs = response.pagination?.total ?? distributedLogs.length;
				expandedId = null;
			}
		} catch (e: unknown) {
			if (requestGeneration !== generation) return;
			const err = e as { message?: string };
			distributedError = err.message || 'Failed to load distributed logs';
		} finally {
			if (requestGeneration === generation) distributedLoading = false;
		}
	}

	function onTabChange(value: string) {
		if (value !== 'this-trace' && value !== 'all-distributed') return;
		activeTab = value;
		if (value === 'all-distributed' && !distributedLoaded && !distributedLoading) {
			loadDistributedLogs();
		}
	}

	function toggleExpanded(id: string) {
		expandedId = expandedId === id ? null : id;
	}

	$effect(() => {
		// SvelteKit reuses this component when navigating between trace URLs.
		const identity = [projectId, traceId, relatedTraceId, traceRecordedAt];
		untrack(() => {
			generation++;
			logs = [];
			error = '';
			logsPage = distributedPage = 1;
			totalLogs = totalDistributedLogs = 0;
			distributedLogs = [];
			distributedLoaded = false;
			distributedLoading = false;
			distributedError = '';
			expandedId = null;
			activeTab = 'this-trace';
			if (identity[0] && identity[1]) loadTraceLogs();
			else loading = false;
		});
		return () => {
			generation++;
		};
	});
</script>

{#snippet pagination(page: number, total: number, busy: boolean, onPage: (page: number) => void)}
	{#if total > 100}
		<div
			class="flex items-center justify-between gap-3 border-t px-6 py-3 text-xs text-muted-foreground"
		>
			<span>Page {page} of {Math.ceil(total / 100)} · {total.toLocaleString()} logs</span>
			<div class="flex gap-2">
				<Button
					variant="outline"
					size="sm"
					disabled={busy || page <= 1}
					onclick={() => onPage(page - 1)}>Previous</Button
				>
				<Button
					variant="outline"
					size="sm"
					disabled={busy || page * 100 >= total}
					onclick={() => onPage(page + 1)}>Next</Button
				>
			</div>
		</div>
	{/if}
{/snippet}

<Card.Root class="gap-0 overflow-hidden pb-0">
	<Card.Header class={relatedTraceId ? '' : 'pb-4'}>
		<Card.Title>Logs</Card.Title>
	</Card.Header>
	<Card.Content class="p-0">
		<Tabs.Root value={activeTab} onValueChange={onTabChange}>
			{#if relatedTraceId}
				<Tabs.List class={cn(underlineTabListClass, 'pb-2 pl-6')}>
					<Tabs.Trigger value="this-trace" class={tabTriggerClass}>This Trace</Tabs.Trigger>
					<Tabs.Trigger value="all-distributed" class={tabTriggerClass}>
						Related Traces
					</Tabs.Trigger>
				</Tabs.List>
			{/if}

			<Tabs.Content value="this-trace" class={relatedTraceId ? 'mt-0' : 'mt-0 border-t'}>
				{#if loading}
					<div class="flex items-center justify-center py-6">
						<LoadingCircle size="lg" />
					</div>
				{:else if error}
					<div class="py-6 text-center text-sm text-red-500">{error}</div>
				{:else if logs.length === 0}
					<div class="py-6 text-center text-sm text-muted-foreground">
						No logs for this trace in the surrounding time window
					</div>
				{:else}
					<Table.Root>
						<Table.Header>
							<Table.Row>
								<Table.Head class="h-8 w-[180px] py-1.5 pl-6">Timestamp</Table.Head>
								<Table.Head class="h-8 w-[80px] py-1.5">Level</Table.Head>
								<Table.Head class="h-8 py-1.5">Message</Table.Head>
								<Table.Head class="h-8 w-[220px] py-1.5 pr-6">Span</Table.Head>
							</Table.Row>
						</Table.Header>
						<Table.Body>
							{#each logs as log (log.id)}
								{@const spanName = resolveSpanName(log)}
								<Table.Row class="h-8 cursor-pointer" onclick={() => toggleExpanded(log.id)}>
									<Table.Cell class="py-1.5 pl-6 text-xs text-muted-foreground tabular-nums">
										{formatDateTime(log.timestamp, { timezone })}
									</Table.Cell>
									<Table.Cell class="py-1.5">
										<SeverityBadge
											severityText={log.severityText}
											severityNumber={log.severityNumber}
										/>
									</Table.Cell>
									<Table.Cell class="max-w-[600px] truncate py-1.5 font-mono text-xs">
										<LogMessage body={log.body} attributes={log.logAttributes} />
									</Table.Cell>
									<Table.Cell class="py-1.5 pr-6">
										{#if spanName}
											<span
												class="inline-block max-w-[200px] truncate rounded bg-muted px-2 py-0.5 font-mono text-xs text-muted-foreground"
												title={spanName}
											>
												{spanName}
											</span>
										{:else}
											<span class="text-xs text-muted-foreground">—</span>
										{/if}
									</Table.Cell>
								</Table.Row>
								{#if expandedId === log.id}
									<ExpandedLogRow {log} colspan={4} />
								{/if}
							{/each}
						</Table.Body>
					</Table.Root>
				{/if}
				{@render pagination(logsPage, totalLogs, loading, loadTraceLogs)}
			</Tabs.Content>

			{#if relatedTraceId}
				<Tabs.Content value="all-distributed" class="mt-0">
					{#if distributedLoading}
						<div class="flex items-center justify-center py-6">
							<LoadingCircle size="lg" />
						</div>
					{:else if distributedError}
						<div class="py-6 text-center text-sm text-red-500">{distributedError}</div>
					{:else if !distributedLoaded}
						<div class="py-6 text-center text-sm text-muted-foreground">
							Loading distributed logs…
						</div>
					{:else if distributedLogs.length === 0}
						<div class="py-6 text-center text-sm text-muted-foreground">
							No logs found in this distributed trace
						</div>
					{:else}
						<Table.Root>
							<Table.Header>
								<Table.Row>
									<Table.Head class="h-8 w-[180px] py-1.5 pl-6">Timestamp</Table.Head>
									<Table.Head class="h-8 w-[80px] py-1.5">Level</Table.Head>
									<Table.Head class="h-8 py-1.5">Message</Table.Head>
									<Table.Head class="h-8 w-[160px] py-1.5">Service</Table.Head>
									<Table.Head class="h-8 w-[130px] py-1.5 pr-6">Trace</Table.Head>
								</Table.Row>
							</Table.Header>
							<Table.Body>
								{#each distributedLogs as log (log.id)}
									<Table.Row class="h-8 cursor-pointer" onclick={() => toggleExpanded(log.id)}>
										<Table.Cell class="py-1.5 pl-6 text-xs text-muted-foreground tabular-nums">
											{formatDateTime(log.timestamp, { timezone })}
										</Table.Cell>
										<Table.Cell class="py-1.5">
											<SeverityBadge
												severityText={log.severityText}
												severityNumber={log.severityNumber}
											/>
										</Table.Cell>
										<Table.Cell class="max-w-[500px] truncate py-1.5 font-mono text-xs">
											<LogMessage body={log.body} attributes={log.logAttributes} />
										</Table.Cell>
										<Table.Cell class="py-1.5 text-xs text-muted-foreground">
											{log.serviceName || '—'}
										</Table.Cell>
										<Table.Cell class="py-1.5 pr-6 font-mono text-xs">
											{#if log.traceId}
												<span class="text-muted-foreground" title={log.traceId}>
													{log.traceId.slice(0, 8)}…
												</span>
											{:else}
												<span class="text-muted-foreground">—</span>
											{/if}
										</Table.Cell>
									</Table.Row>
									{#if expandedId === log.id}
										<ExpandedLogRow {log} colspan={5} />
									{/if}
								{/each}
							</Table.Body>
						</Table.Root>
					{/if}
					{@render pagination(
						distributedPage,
						totalDistributedLogs,
						distributedLoading,
						loadDistributedLogs
					)}
				</Tabs.Content>
			{/if}
		</Tabs.Root>
	</Card.Content>
</Card.Root>
