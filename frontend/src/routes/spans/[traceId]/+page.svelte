<script lang="ts">
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import { api } from '$lib/api';
	import { getErrorMessage, getErrorStatus } from '$lib/utils/errors';
	import { projectsState, isFrontendFramework } from '$lib/state/projects.svelte';
	import { getTimezone } from '$lib/state/timezone.svelte';
	import { formatDateTime, formatDuration } from '$lib/utils/formatters';
	import { createSmartBackHandler } from '$lib/utils/back-navigation';
	import { summarizeSpanTrace } from '$lib/utils/span-explorer';
	import * as Card from '$lib/components/ui/card';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { ErrorDisplay } from '$lib/components/ui/error-display';
	import CopyButton from '$lib/components/traceway/copy-button.svelte';
	import PageHeader from '$lib/components/traceway/page-header.svelte';
	import SpanWaterfall from '$lib/components/spans/span-waterfall.svelte';
	import SpanGraphNotice from '$lib/components/spans/span-graph-notice.svelte';
	import TraceLogsPanel from '$lib/components/trace-logs/trace-logs-panel.svelte';
	import type { Span, SpanAttributes, SpanGraphStatus } from '$lib/types/spans';

	let { data } = $props();

	type SpanTraceResponse = {
		spanGraphStatus?: SpanGraphStatus | null;
		spans: Span[] | null;
		projects?: { id: string; name: string }[] | null;
	};

	const timezone = $derived(getTimezone());
	const onBack = createSmartBackHandler({ fallbackPath: resolve('/spans') });

	// Raw on purpose: a deep proxy over 20,000 spans took minutes to read. The array is only ever replaced.
	let spans = $state.raw<Span[]>([]);
	let status = $state<SpanGraphStatus | null>(null);
	// The trace is read from every project of the organization, so a row may live in another project than this page.
	let projectNames = $state.raw<Record<string, string>>({});
	const projectCount = $derived(Object.keys(projectNames).length);
	let loading = $state(true);
	let error = $state('');
	let notFound = $state(false);

	const summary = $derived(summarizeSpanTrace(spans));
	const showLogs = $derived(
		!!projectsState.currentProject && !isFrontendFramework(projectsState.currentProject.framework)
	);

	const truncated = $derived(!!status?.reasons?.includes('most_important'));

	function loadAttributes(span: Span): Promise<SpanAttributes> {
		return api.get(
			`/spans/traces/${data.traceId}/spans/${span.spanId}/attributes?at=${encodeURIComponent(span.startTime)}`,
			{ projectId: span.projectId }
		);
	}

	async function loadData() {
		loading = true;
		error = '';
		notFound = false;
		if (!data.at) {
			notFound = true;
			loading = false;
			return;
		}
		try {
			const response = (await api.get(
				`/spans/traces/${data.traceId}?at=${encodeURIComponent(data.at)}`,
				{ projectId: projectsState.currentProjectId ?? undefined }
			)) as SpanTraceResponse;
			spans = response.spans || [];
			status = response.spanGraphStatus ?? null;
			projectNames = Object.fromEntries((response.projects ?? []).map((p) => [p.id, p.name]));
		} catch (e) {
			console.error(e);
			if (getErrorStatus(e) === 404) {
				notFound = true;
			} else {
				error = getErrorMessage(e) || 'Failed to load the trace';
			}
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		loadData();
	});
</script>

<div class="space-y-6">
	<PageHeader title="Trace" {onBack}>
		{#snippet meta()}
			<div class="flex items-center gap-1 font-mono text-sm text-muted-foreground">
				<span class="break-all">{data.traceId}</span>
				<CopyButton bare text={data.traceId} iconClass="h-3.5 w-3.5" class="shrink-0" />
			</div>
		{/snippet}
	</PageHeader>

	{#if loading}
		<div class="flex items-center justify-center py-20">
			<LoadingCircle size="xlg" />
		</div>
	{:else if notFound}
		<ErrorDisplay
			status={404}
			title="Trace Not Found"
			description="No spans of this trace were recorded within 24 hours of the linked time. It may have expired, or the link may be incomplete."
			{onBack}
			backLabel="Back to Spans"
			onRetry={loadData}
			identifier={data.traceId}
		/>
	{:else if error}
		<ErrorDisplay
			status={400}
			title="Failed to Load Trace"
			description={error}
			{onBack}
			backLabel="Back to Spans"
			onRetry={loadData}
		/>
	{:else}
		<Card.Root>
			<Card.Header>
				<Card.Title>Spans</Card.Title>
				<Card.Description>
					{spans.length.toLocaleString()}
					{spans.length === 1 ? 'span' : 'spans'} across {summary.services}
					{summary.services === 1 ? 'service' : 'services'}{projectCount > 1
						? ` in ${projectCount} projects`
						: ''}, {formatDuration(summary.duration)} end to end{#if summary.startTime}, started {formatDateTime(
							summary.startTime,
							{
								timezone,
								format: 'datetime-seconds'
							}
						)}{/if}{#if summary.errors > 0}<span class="text-destructive"
							>, {summary.errors}
							{summary.errors === 1 ? 'error' : 'errors'}</span
						>{/if}
				</Card.Description>
			</Card.Header>
			<Card.Content class="space-y-3">
				<SpanGraphNotice {status} />
				{#if spans.length > 0}
					<SpanWaterfall
						{spans}
						traceDuration={summary.duration}
						traceStartTime={summary.startTime}
						linkTraces={false}
						{loadAttributes}
						{projectNames}
						missingParentLabel={truncated ? 'Parent span is not among the spans shown' : undefined}
						selectedSpanId={data.spanId}
					/>
				{/if}
			</Card.Content>
		</Card.Root>

		{#if showLogs && spans.length > 0}
			<TraceLogsPanel
				projectId={projectsState.currentProjectId ?? ''}
				traceId={data.traceId}
				{spans}
				traceRecordedAt={summary.startTime}
			/>
		{/if}
	{/if}
</div>
