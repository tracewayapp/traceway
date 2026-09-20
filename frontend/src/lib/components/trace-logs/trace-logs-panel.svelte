<script lang="ts">
	import { onMount } from 'svelte';
	import { SvelteMap } from 'svelte/reactivity';
	import { api } from '$lib/api';
	import * as Card from '$lib/components/ui/card';
	import * as Table from '$lib/components/ui/table';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { SeverityBadge } from '$lib/components/ui/severity-badge';
	import ExpandedLogRow from './expanded-log-row.svelte';
	import LogMessage from './log-message.svelte';
	import { formatDateTime } from '$lib/utils/formatters';
	import { getTimezone } from '$lib/state/timezone.svelte';
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
		spans,
		traceRecordedAt
	}: {
		projectId: string;
		traceId: string;
		spans: Span[];
		traceRecordedAt: string;
	} = $props();

	const timezone = $derived(getTimezone());

	let logs = $state<LogRecord[]>([]);
	let loading = $state(true);
	let error = $state('');
	let expandedId = $state<string | null>(null);

	const childSpanNameByHex = $derived.by(() => {
		const m = new SvelteMap<string, string>();
		for (const s of spans) m.set(s.spanId, s.name);
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

	function toggleExpanded(id: string) {
		expandedId = expandedId === id ? null : id;
	}

	onMount(() => {
		loadTraceLogs();
	});
</script>

<Card.Root class="gap-0 overflow-hidden pb-0">
	<Card.Header class="pb-4">
		<Card.Title>Logs</Card.Title>
	</Card.Header>
	<Card.Content class="border-t p-0">
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
	</Card.Content>
</Card.Root>
