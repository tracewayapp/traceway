<script lang="ts">
	import { getErrorMessage, getErrorStatus } from '$lib/utils/errors';
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import { page as pageState } from '$app/state';
	import { api } from '$lib/api';
	import { projectsState } from '$lib/state/projects.svelte';
	import { getTimezone } from '$lib/state/timezone.svelte';
	import { formatDateTime } from '$lib/utils/formatters';
	import { sessionDurationLabel } from '$lib/utils/session-duration';
	import { createSmartBackHandler } from '$lib/utils/back-navigation';
	import { createRowClickHandler } from '$lib/utils/navigation';
	import * as Card from '$lib/components/ui/card';
	import * as Table from '$lib/components/ui/table';
	import * as Tabs from '$lib/components/ui/tabs';
	import { Button } from '$lib/components/ui/button';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { LabelValue } from '$lib/components/ui/label-value';
	import { AttributesGrid } from '$lib/components/ui/attributes-grid';
	import { TableEmptyState } from '$lib/components/ui/table-empty-state';
	import SessionReplay from '$lib/components/issues/session-replay.svelte';
	import SessionLogsTable from '$lib/components/issues/session-logs-table.svelte';
	import SessionActionsTable from '$lib/components/issues/session-actions-table.svelte';
	import { ErrorDisplay } from '$lib/components/ui/error-display';
	import PageHeader from '$lib/components/traceway/page-header.svelte';
	import TableContainer from '$lib/components/traceway/table-container.svelte';
	import { ArrowRight, Radar } from '@lucide/svelte';
	import SdkCaptureDialog from '$lib/components/sdk-config/sdk-capture-dialog.svelte';
	import { SDK_CONFIG_FRAMEWORKS } from '$lib/state/projects.svelte';
	import type {
		SessionActionEvent,
		SessionLogEvent,
		SessionReplayEvent
	} from '$lib/types/exceptions';
	import {
		loadSessionTimeline,
		pickBreadcrumbs,
		type SessionBreadcrumbs
	} from '$lib/utils/session-timeline';

	type Session = {
		id: string;
		startedAt: string;
		endedAt?: string | null;
		duration: number;
		hasRecording?: boolean;
		clientIP: string;
		appVersion: string;
		serverName: string;
		attributes?: Record<string, string> | null;
	};

	type SessionException = {
		id: string;
		exceptionHash: string;
		stackTrace: string;
		recordedAt: string;
		isMessage: boolean;
	};

	const timezone = $derived(getTimezone());
	const sessionId = $derived(pageState.params.sessionId);

	let session = $state<Session | null>(null);
	let exceptions = $state<SessionException[]>([]);
	let recordingEvents = $state<SessionReplayEvent[] | null>(null);
	let recordingLogs = $state<SessionLogEvent[]>([]);
	let recordingActions = $state<SessionActionEvent[]>([]);
	let timeline = $state<SessionBreadcrumbs | null>(null);
	let recordingLoading = $state(false);
	let recordingError = $state('');
	let loading = $state(true);
	let error = $state('');
	let notFound = $state(false);

	let currentTimeMs = $state(0);
	let replayRef = $state<{ seek: (ms: number) => void } | null>(null);

	function handleSeek(offsetMs: number) {
		replayRef?.seek(offsetMs);
	}

	const breadcrumbs = $derived(
		pickBreadcrumbs(timeline, { logs: recordingLogs, actions: recordingActions })
	);
	const hasLogs = $derived(breadcrumbs.logs.length > 0);
	const hasActions = $derived(breadcrumbs.actions.length > 0);
	const defaultTab = $derived(hasLogs ? 'logs' : 'actions');

	let loadGeneration = 0;

	function requestRecording(startedAt: string) {
		return api.get(`/sessions/${sessionId}/recording?startedAt=${encodeURIComponent(startedAt)}`, {
			projectId: projectsState.currentProjectId ?? undefined
		});
	}

	async function loadRecording(generation: number, request: ReturnType<typeof requestRecording>) {
		recordingLoading = true;
		recordingError = '';
		try {
			const recording = await request;
			if (generation !== loadGeneration) return;
			recordingEvents = recording?.events ?? [];
			recordingLogs = recording?.logs ?? [];
			recordingActions = recording?.actions ?? [];
		} catch (e) {
			if (generation !== loadGeneration) return;
			recordingError = getErrorMessage(e) || 'Failed to load the replay';
		} finally {
			if (generation === loadGeneration) recordingLoading = false;
		}
	}

	// The SDK stamps breadcrumbs with the client clock, so the window leaves a minute either side.
	async function loadTimeline(generation: number, loaded: Session) {
		const start = Date.parse(loaded.startedAt);
		const end = loaded.endedAt ? Date.parse(loaded.endedAt) : Date.now();
		const result = await loadSessionTimeline(
			loaded.id,
			start - 60_000,
			end + 60_000,
			projectsState.currentProjectId ?? undefined
		);
		if (generation === loadGeneration) timeline = result;
	}

	function retryRecording() {
		if (!session) return;
		loadRecording(++loadGeneration, requestRecording(session.startedAt));
	}

	async function loadAll() {
		const generation = ++loadGeneration;
		loading = true;
		error = '';
		notFound = false;
		recordingEvents = null;
		recordingLogs = [];
		recordingActions = [];
		timeline = null;

		const startedAt = pageState.url.searchParams.get('t');
		const recordingRequest = startedAt ? requestRecording(startedAt) : null;
		recordingRequest?.catch(() => {});

		let loaded: Session;
		try {
			const detail = await api.post(`/sessions/${sessionId}`, startedAt ? { startedAt } : {}, {
				projectId: projectsState.currentProjectId ?? undefined
			});
			if (generation !== loadGeneration) return;
			loaded = detail.session;
			session = loaded;
			exceptions = detail.exceptions || [];
		} catch (e) {
			if (generation !== loadGeneration) return;
			if (getErrorStatus(e) === 404) {
				notFound = true;
			} else {
				error = getErrorMessage(e) || 'Failed to load session';
			}
			return;
		} finally {
			if (generation === loadGeneration) loading = false;
		}

		loadRecording(generation, recordingRequest ?? requestRecording(loaded.startedAt));
		loadTimeline(generation, loaded);
	}

	onMount(() => {
		loadAll();
	});

	let captureOpen = $state(false);
	const canCapture = $derived(
		!!projectsState.currentProject &&
			SDK_CONFIG_FRAMEWORKS.includes(projectsState.currentProject.framework) &&
			projectsState.canWriteCurrentProject
	);
	const sdkVersion = $derived(session?.attributes?.['traceway.sdk.version']);
	const sdkLabel = $derived(
		sdkVersion
			? `JS SDK v${sdkVersion}`
			: session?.attributes?.['traceway.sdk.generation'] === '1'
				? 'V1 · legacy ingestion'
				: 'Unknown · not reported'
	);
	// A device keeps its instance id across sessions, so capturing it also covers the visits that follow.
	const captureTarget = $derived(
		session?.attributes?.['service.instance.id']
			? { kind: 'instance' as const, value: session.attributes['service.instance.id'] }
			: { kind: 'session' as const, value: session?.id ?? '' }
	);
</script>

<div class="space-y-6">
	<PageHeader
		title="Session"
		onBack={createSmartBackHandler({ fallbackPath: resolve('/sessions') })}
	>
		{#snippet actions()}
			{#if canCapture && session}
				<Button variant="outline" size="sm" onclick={() => (captureOpen = true)}>
					<Radar class="mr-2 h-4 w-4" />
					Capture this {captureTarget.kind === 'instance' ? 'device' : 'session'}
				</Button>
			{/if}
		{/snippet}
	</PageHeader>

	{#if canCapture}
		<SdkCaptureDialog
			bind:open={captureOpen}
			kind={captureTarget.kind}
			value={captureTarget.value}
		/>
	{/if}

	{#if loading}
		<div class="flex h-64 items-center justify-center">
			<LoadingCircle size="xlg" />
		</div>
	{:else if notFound}
		<ErrorDisplay
			status={404}
			title="Session not found"
			description="This session does not exist or has been pruned."
			onRetry={loadAll}
		/>
	{:else if error}
		<ErrorDisplay status={500} title="Error" description={error} onRetry={loadAll} />
	{:else if session}
		<Card.Root>
			<Card.Header>
				<Card.Title>Overview</Card.Title>
			</Card.Header>
			<Card.Content class="grid grid-cols-1 gap-4 sm:grid-cols-3">
				<LabelValue label="Started" value={formatDateTime(session.startedAt, { timezone })} />
				<LabelValue label="Duration" value={sessionDurationLabel(session)} />
				<LabelValue label="App version" value={session.appVersion || '—'} />
				{#if projectsState.currentProject && SDK_CONFIG_FRAMEWORKS.includes(projectsState.currentProject.framework)}
					<LabelValue label="JavaScript SDK" value={sdkLabel} />
				{/if}
			</Card.Content>
		</Card.Root>

		{#if session.attributes && Object.keys(session.attributes).length > 0}
			<Card.Root>
				<Card.Header>
					<Card.Title>Attributes</Card.Title>
				</Card.Header>
				<Card.Content>
					<AttributesGrid attributes={session.attributes} collapsedCount={6} />
				</Card.Content>
			</Card.Root>
		{/if}

		{#if recordingLoading}
			<Card.Root>
				<Card.Header>
					<Card.Title>Session Replay</Card.Title>
				</Card.Header>
				<Card.Content class="flex h-64 items-center justify-center">
					<LoadingCircle size="lg" />
				</Card.Content>
			</Card.Root>
		{:else if recordingError}
			<Card.Root>
				<Card.Header>
					<Card.Title>Session Replay</Card.Title>
				</Card.Header>
				<Card.Content class="flex items-center justify-between gap-4 text-sm text-muted-foreground">
					<span>{recordingError}</span>
					<Button variant="outline" size="sm" onclick={retryRecording}>Retry</Button>
				</Card.Content>
			</Card.Root>
		{:else if recordingEvents && recordingEvents.length > 0}
			<Card.Root class="pb-0">
				<Card.Header class="gap-0">
					<Card.Title>Session Replay</Card.Title>
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
		{:else}
			<Card.Root>
				<Card.Header>
					<Card.Title>Session Replay</Card.Title>
				</Card.Header>
				<Card.Content class="text-sm text-muted-foreground">
					No replay data has been uploaded for this session yet.
				</Card.Content>
			</Card.Root>
		{/if}

		{#if hasLogs || hasActions}
			<Card.Root>
				<Card.Header>
					<Card.Title>Session Context</Card.Title>
					<Card.Description>
						Console output, requests, navigation and app events across the session. Times are
						offsets from session start.{#if breadcrumbs.truncated}
							Showing the first {breadcrumbs.logs.length} log lines and
							{breadcrumbs.actions.length} actions.{/if}
					</Card.Description>
				</Card.Header>
				<Card.Content>
					<Tabs.Root value={defaultTab}>
						<Tabs.List class="mb-4">
							{#if hasLogs}
								<Tabs.Trigger value="logs">Logs ({breadcrumbs.logs.length})</Tabs.Trigger>
							{/if}
							{#if hasActions}
								<Tabs.Trigger value="actions">Actions ({breadcrumbs.actions.length})</Tabs.Trigger>
							{/if}
						</Tabs.List>
						{#if hasLogs}
							<Tabs.Content value="logs">
								<SessionLogsTable
									logs={breadcrumbs.logs}
									startedAt={session.startedAt}
									{currentTimeMs}
									onSeek={handleSeek}
								/>
							</Tabs.Content>
						{/if}
						{#if hasActions}
							<Tabs.Content value="actions">
								<SessionActionsTable
									actions={breadcrumbs.actions}
									startedAt={session.startedAt}
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
				<Card.Title>Exceptions in this session</Card.Title>
			</Card.Header>
			<Card.Content>
				<TableContainer>
					<Table.Root>
						{#if exceptions.length === 0}
							<Table.Body>
								<TableEmptyState
									colspan={3}
									message="No exceptions captured during this session."
								/>
							</Table.Body>
						{:else}
							<Table.Header>
								<Table.Row>
									<Table.Head>When</Table.Head>
									<Table.Head>Stack trace</Table.Head>
									<Table.Head class="w-[60px]"></Table.Head>
								</Table.Row>
							</Table.Header>
							<Table.Body>
								{#each exceptions as exc, __index (__index)}
									<Table.Row
										class="cursor-pointer"
										onclick={createRowClickHandler(`/issues/${exc.exceptionHash}`)}
									>
										<Table.Cell class="text-sm whitespace-nowrap"
											>{formatDateTime(exc.recordedAt, { timezone })}</Table.Cell
										>
										<Table.Cell class="max-w-md truncate font-mono text-xs">
											{exc.stackTrace.split('\n')[0]}
										</Table.Cell>
										<Table.Cell>
											<Button variant="ghost" size="icon" aria-label="Open issue">
												<ArrowRight class="size-4" />
											</Button>
										</Table.Cell>
									</Table.Row>
								{/each}
							</Table.Body>
						{/if}
					</Table.Root>
				</TableContainer>
			</Card.Content>
		</Card.Root>
	{/if}
</div>
