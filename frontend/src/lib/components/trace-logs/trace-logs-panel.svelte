<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api';
	import * as Card from '$lib/components/ui/card';
	import * as Table from '$lib/components/ui/table';
	import * as Tabs from '$lib/components/ui/tabs';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { SeverityBadge } from '$lib/components/ui/severity-badge';
	import ExpandedLogRow from './expanded-log-row.svelte';
	import LogMessage from './log-message.svelte';
	import { formatDateTime } from '$lib/utils/formatters';
	import { getTimezone } from '$lib/state/timezone.svelte';
	import { underlineTabTriggerClass, underlineTabListClass } from '$lib/utils/tabs';
	import { cn } from '$lib/utils';
	import { spanIdUuidToHex } from '$lib/utils/span-id';
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
		distributedTraceId = null,
		spans,
		traceRecordedAt
	}: {
		projectId: string;
		traceId: string;
		distributedTraceId?: string | null;
		spans: Span[];
		rootSpan: { id: string; name: string };
		traceRecordedAt: string;
	} = $props();

	const timezone = $derived(getTimezone());

	let activeTab = $state<'this-trace' | 'all-distributed'>('this-trace');

	const tabTriggerClass = cn(underlineTabTriggerClass, 'mb-0 pb-2');

	// This-trace data
	let logs = $state<LogRecord[]>([]);
	let loading = $state(true);
	let error = $state('');
	let expandedId = $state<string | null>(null);

	// Distributed data (lazy)
	let distributedLogs = $state<LogRecord[]>([]);
	let distributedLoading = $state(false);
	let distributedLoaded = $state(false);
	let distributedError = $state('');

	const childSpanNameByHex = $derived.by(() => {
		const m = new Map<string, string>();
		for (const s of spans) {
			const hex = spanIdUuidToHex(s.id);
			if (hex) m.set(hex, s.name);
		}
		return m;
	});

	function resolveSpanName(log: LogRecord): string | null {
		if (!log.spanId) return null;
		return childSpanNameByHex.get(log.spanId.toLowerCase()) ?? null;
	}

	function timeWindow(): { fromDate: string; toDate: string } {
		const t = new Date(traceRecordedAt).getTime();
		const hour = 60 * 60 * 1000;
		return {
			fromDate: new Date(t - hour).toISOString(),
			toDate: new Date(t + hour).toISOString()
		};
	}

	async function loadTraceLogs() {
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
					pagination: { page: 1, pageSize: 100 },
					traceId
				},
				{ projectId: projectId || undefined }
			)) as { data: LogRecord[] };
			logs = response.data || [];
		} catch (e: unknown) {
			const err = e as { message?: string };
			error = err.message || 'Failed to load logs';
		} finally {
			loading = false;
		}
	}

	async function loadDistributedLogs() {
		if (!distributedTraceId) return;
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
					pagination: { page: 1, pageSize: 100 },
					distributedTraceId
				},
				{ projectId: projectId || undefined }
			)) as { data: LogRecord[] };
			distributedLogs = response.data || [];
			distributedLoaded = true;
		} catch (e: unknown) {
			const err = e as { message?: string };
			distributedError = err.message || 'Failed to load distributed logs';
		} finally {
			distributedLoading = false;
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

	onMount(() => {
		loadTraceLogs();
	});
</script>

<Card.Root class="gap-0 overflow-hidden pb-0">
	<Card.Header class={distributedTraceId ? '' : 'pb-4'}>
		<Card.Title>Logs</Card.Title>
	</Card.Header>
	<Card.Content class="p-0">
		<Tabs.Root value={activeTab} onValueChange={onTabChange}>
			{#if distributedTraceId}
				<Tabs.List class={cn(underlineTabListClass, 'pb-2 pl-6')}>
					<Tabs.Trigger value="this-trace" class={tabTriggerClass}>This Trace</Tabs.Trigger>
					<Tabs.Trigger value="all-distributed" class={tabTriggerClass}>
						All Distributed Traces
					</Tabs.Trigger>
				</Tabs.List>
			{/if}

			<Tabs.Content value="this-trace" class={distributedTraceId ? 'mt-0' : 'mt-0 border-t'}>
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
			</Tabs.Content>

			{#if distributedTraceId}
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
				</Tabs.Content>
			{/if}
		</Tabs.Root>
	</Card.Content>
</Card.Root>
