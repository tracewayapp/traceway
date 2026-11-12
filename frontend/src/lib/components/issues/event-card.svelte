<script lang="ts">
	import { resolve } from '$app/paths';
	import { goto } from '$app/navigation';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import * as Tabs from '$lib/components/ui/tabs';
	import { ArrowRight, Radar } from '@lucide/svelte';
	import SdkCaptureDialog from '$lib/components/sdk-config/sdk-capture-dialog.svelte';
	import type { ExceptionOccurrence, LinkedTrace, SessionRecording } from '$lib/types/exceptions';
	import { formatDuration, getStatusColor, formatDateTime } from '$lib/utils/formatters';
	import { getTimezone } from '$lib/state/timezone.svelte';
	import { AttributesGrid } from '$lib/components/ui/attributes-grid';
	import { LabelValue } from '../ui/label-value';
	import {
		projectsState,
		isFrontendFramework,
		SDK_CONFIG_FRAMEWORKS
	} from '$lib/state/projects.svelte';
	import SessionReplay from './session-replay.svelte';
	import SessionLogsTable from './session-logs-table.svelte';
	import SessionActionsTable from './session-actions-table.svelte';
	import DistributedTraceCard from '$lib/components/distributed-trace/distributed-trace-card.svelte';
	import {
		loadSessionTimeline,
		pickBreadcrumbs,
		type SessionBreadcrumbs
	} from '$lib/utils/session-timeline';

	interface Props {
		occurrence: ExceptionOccurrence;
		linkedTrace: LinkedTrace | null;
		sessionRecording?: SessionRecording | null;
		sessionId?: string | null;
		title?: string;
		description?: string;
		timezone?: string;
	}

	let {
		occurrence,
		linkedTrace,
		sessionRecording = null,
		sessionId = null,
		title = 'Event',
		description = 'Details for this specific occurrence',
		timezone
	}: Props = $props();

	const RELATED_LABELS = {
		endpoint: { label: 'Endpoint', route: 'endpoints' },
		task: { label: 'Task', route: 'tasks' },
		ai_trace: { label: 'AI Trace', route: 'ai-traces' }
	} as const;

	const tz = $derived(timezone ?? getTimezone());
	const isFrontend = $derived(
		projectsState.currentProject?.framework
			? isFrontendFramework(projectsState.currentProject.framework)
			: false
	);

	const recordingEvents = $derived(sessionRecording?.events ?? null);
	const hasReplay = $derived(!!recordingEvents && recordingEvents.length > 0);
	const replayStatus = $derived(occurrence.attributes?.['traceway.replay.status']);
	const replayExplanation = $derived(
		(
			{
				disabled: 'Replay recording was disabled by SDK settings, a remote rule, or plan limits.',
				cooldown:
					'This error occurred during the configured replay cooldown. Open the session to inspect the nearby clip.',
				session_clip_limit:
					'This session reached its replay clip limit. Breadcrumbs are still captured within collection limits.',
				trigger_disabled: 'The error replay trigger was disabled.',
				sampled_out: 'The replay trigger sampled out this occurrence.',
				collection_limit:
					'The session reached its diagnostic collection budget. Errors and web vitals continue.',
				buffer_unavailable:
					'No replay segment was available. Recording may not have started yet or the buffer may have been evicted.',
				requested:
					'The SDK requested a replay upload, but no recording is available here yet. The upload may be pending or may have failed.',
				not_triggered: 'No replay trigger matched this error.'
			} as Record<string, string>
		)[replayStatus ?? ''] ?? 'No recording is available. This SDK did not report why.'
	);
	let timeline = $state<SessionBreadcrumbs | null>(null);
	$effect(() => {
		const id = sessionId;
		const at = Date.parse(occurrence.recordedAt);
		timeline = null;
		if (!id || !isFrontend || !Number.isFinite(at)) return;
		let cancelled = false;
		loadSessionTimeline(
			id,
			at - 600_000,
			at + 1_000,
			projectsState.currentProjectId ?? undefined,
			'end'
		).then((result) => {
			if (!cancelled) timeline = result;
		});
		return () => {
			cancelled = true;
		};
	});
	const breadcrumbs = $derived(pickBreadcrumbs(timeline, sessionRecording));
	// A read cut short at its limit kept the latest breadcrumbs; the oldest of them is where the list becomes complete.
	const truncationNote = $derived.by(() => {
		const rows = [...(timeline?.logs ?? []), ...(timeline?.actions ?? [])];
		if (!timeline?.truncated || rows.length === 0) return '';
		const earliest = rows.reduce((min, row) => Math.min(min, Date.parse(row.timestamp)), Infinity);
		const from = formatDateTime(earliest, { timezone: tz, format: 'datetime-seconds' });
		return `That window holds more than can be shown, so only the latest are listed, from ${from}.`;
	});
	const logs = $derived(breadcrumbs.logs);
	const actions = $derived(breadcrumbs.actions);
	// Without a replay to seek, times read best relative to the exception.
	const timelineOrigin = $derived(sessionRecording?.startedAt ?? occurrence.recordedAt);
	const hasLogs = $derived(logs.length > 0);
	const hasActions = $derived(actions.length > 0);
	// Logs/actions tables are only meaningful for frontend-framework projects.
	// Backend / OTel exception pages must look identical to before.
	const showSessionContext = $derived(isFrontend && (hasLogs || hasActions));
	const defaultTab = $derived(hasLogs ? 'logs' : 'actions');

	let currentTimeMs = $state(0);
	let replayRef = $state<{ seek: (ms: number) => void } | null>(null);

	function handleSeek(offsetMs: number) {
		replayRef?.seek(offsetMs);
	}

	let captureOpen = $state(false);
	const canCapture = $derived(
		!!projectsState.currentProject &&
			SDK_CONFIG_FRAMEWORKS.includes(projectsState.currentProject.framework) &&
			projectsState.canWriteCurrentProject
	);
	const captureTarget = $derived.by(() => {
		const attrs = occurrence.attributes ?? {};
		if (attrs['service.instance.id']) {
			return { kind: 'instance' as const, value: attrs['service.instance.id'], label: 'device' };
		}
		if (attrs['user.id']) return { kind: 'user' as const, value: attrs['user.id'], label: 'user' };
		return sessionId ? { kind: 'session' as const, value: sessionId, label: 'session' } : null;
	});
</script>

{#snippet sessionActions()}
	<div class="flex items-center gap-2">
		{#if canCapture && captureTarget}
			<Button variant="outline" size="sm" onclick={() => (captureOpen = true)}>
				<Radar class="size-4" />
				Capture {captureTarget.label}
			</Button>
		{/if}
		<Button variant="outline" size="sm" onclick={() => goto(resolve(`/sessions/${sessionId}`))}>
			View full session
			<ArrowRight class="size-4" />
		</Button>
	</div>
{/snippet}

{#if canCapture && captureTarget}
	<SdkCaptureDialog bind:open={captureOpen} kind={captureTarget.kind} value={captureTarget.value} />
{/if}

{#if isFrontend && (Number(occurrence.attributes?.['traceway.context.evicted']) > 0 || Number(occurrence.attributes?.['traceway.collection.dropped']) > 0)}
	<p class="text-sm text-muted-foreground" role="status">
		This context may be incomplete. The SDK reported {occurrence.attributes?.[
			'traceway.context.evicted'
		] ?? '0'} buffer evictions and {occurrence.attributes?.['traceway.collection.dropped'] ?? '0'} diagnostic
		events dropped at its collection limit.
	</p>
{/if}

{#if isFrontend && !hasReplay}
	<div class="rounded-md border p-4 text-sm" role="status">
		<p class="font-medium">Replay unavailable</p>
		<p class="mt-1 text-muted-foreground">{replayExplanation}</p>
		{#if sessionId}
			<Button variant="link" class="px-0" onclick={() => goto(resolve(`/sessions/${sessionId}`))}
				>Open session context</Button
			>
		{/if}
	</div>
{/if}

{#if hasReplay}
	<Card.Root class="pb-0">
		<Card.Header class="flex flex-row items-center justify-between gap-2">
			<Card.Title>Session Replay</Card.Title>
			{#if sessionId}
				{@render sessionActions()}
			{/if}
		</Card.Header>
		<Card.Content class="p-0">
			{#key recordingEvents}
				<SessionReplay
					bind:this={replayRef}
					events={recordingEvents}
					onTimeUpdate={(ms) => (currentTimeMs = ms)}
				/>
			{/key}
		</Card.Content>
	</Card.Root>
{:else if sessionId}
	<Card.Root>
		<Card.Header class="flex flex-row items-center justify-between gap-2">
			<Card.Title>Session</Card.Title>
			{@render sessionActions()}
		</Card.Header>
		<Card.Content class="text-sm text-muted-foreground">
			This exception is linked to a user session.
		</Card.Content>
	</Card.Root>
{/if}

<!-- Session Context (logs + actions). Frontend projects only. -->
{#if showSessionContext}
	<Card.Root>
		<Card.Header>
			<Card.Title>Session Context</Card.Title>
			<Card.Description>
				Console output, requests, navigation and app events in the 10 minutes before this exception.
				Times are offsets from {hasReplay ? 'the start of the recording' : 'the exception'}.
				{truncationNote}
			</Card.Description>
		</Card.Header>
		<Card.Content>
			<Tabs.Root value={defaultTab}>
				<Tabs.List class="mb-4">
					{#if hasLogs}
						<Tabs.Trigger value="logs">Logs ({logs.length})</Tabs.Trigger>
					{/if}
					{#if hasActions}
						<Tabs.Trigger value="actions">Actions ({actions.length})</Tabs.Trigger>
					{/if}
				</Tabs.List>
				{#if hasLogs}
					<Tabs.Content value="logs">
						<SessionLogsTable
							{logs}
							startedAt={timelineOrigin}
							{currentTimeMs}
							onSeek={handleSeek}
						/>
					</Tabs.Content>
				{/if}
				{#if hasActions}
					<Tabs.Content value="actions">
						<SessionActionsTable
							{actions}
							startedAt={timelineOrigin}
							{currentTimeMs}
							onSeek={handleSeek}
						/>
					</Tabs.Content>
				{/if}
			</Tabs.Root>
		</Card.Content>
	</Card.Root>
{/if}

<Card.Root>
	<Card.Header>
		<Card.Title>{title}</Card.Title>
		<Card.Description>{description}</Card.Description>
	</Card.Header>
	<Card.Content class="space-y-6">
		<!-- Event Details -->
		<div class="grid grid-cols-2 gap-4 {isFrontend ? 'md:grid-cols-2' : 'md:grid-cols-4'}">
			<div class="space-y-1">
				<p class="text-sm text-muted-foreground">Recorded At</p>
				<p class="font-mono text-sm">{formatDateTime(occurrence.recordedAt, { timezone: tz })}</p>
			</div>
			{#if !isFrontend}
				<div class="space-y-1">
					<p class="text-sm text-muted-foreground">Server</p>
					<p class="font-mono text-sm">{occurrence.serverName || '-'}</p>
				</div>
			{/if}
			<div class="space-y-1">
				<p class="text-sm text-muted-foreground">Version</p>
				<p class="font-mono text-sm">{occurrence.appVersion || '-'}</p>
			</div>
			{#if !isFrontend}
				<div class="space-y-1">
					<p class="text-sm text-muted-foreground">Trace</p>
					<p class="font-mono text-sm">{occurrence.traceId || '-'}</p>
				</div>
			{/if}
		</div>

		<!-- AttributesGrid -->
		{#if occurrence.attributes && Object.keys(occurrence.attributes).length > 0}
			<hr class="border-border" />
			<div>
				<p class="mb-3 text-sm font-medium">Attributes</p>
				<AttributesGrid attributes={occurrence.attributes} />
			</div>
		{/if}

		<!-- Related Trace -->
		{#if linkedTrace}
			{@const related = RELATED_LABELS[linkedTrace.traceType]}
			<hr class="border-border" />
			<div>
				<p class="mb-3 text-sm font-medium">Related {related.label}</p>
				<div class="mb-4 grid grid-cols-2 gap-4 md:grid-cols-4">
					<LabelValue label={related.label} value={linkedTrace.endpoint} mono />
					{#if linkedTrace.traceType === 'endpoint'}
						<LabelValue
							label="Status"
							value={linkedTrace.statusCode}
							mono
							large
							valueClass={getStatusColor(linkedTrace.statusCode)}
						/>
					{/if}
					<LabelValue label="Duration" value={formatDuration(linkedTrace.duration)} mono large />
					<LabelValue
						label="Recorded At"
						value={formatDateTime(linkedTrace.recordedAt, { timezone })}
						mono
					/>
				</div>
				<Button
					variant="outline"
					size="sm"
					onclick={() =>
						goto(
							resolve(
								`/${related.route}/${encodeURIComponent(linkedTrace.endpoint)}/${linkedTrace.id}?preset=24h&t=${encodeURIComponent(linkedTrace.recordedAt)}`
							)
						)}
				>
					View {related.label} Details
					<ArrowRight class="ml-2 h-4 w-4" />
				</Button>
			</div>
		{/if}
	</Card.Content>
</Card.Root>

{#if occurrence.traceId}
	<DistributedTraceCard
		traceId={occurrence.traceId}
		currentExceptionHash={occurrence.exceptionHash}
		recordedAt={linkedTrace?.recordedAt ?? occurrence.recordedAt}
	/>
{/if}
