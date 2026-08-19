<script lang="ts">
	import { getErrorMessage, getErrorStatus } from '$lib/utils/errors';
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import { page as pageState } from '$app/state';
	import { api } from '$lib/api';
	import { projectsState } from '$lib/state/projects.svelte';
	import { getTimezone } from '$lib/state/timezone.svelte';
	import { formatDuration, formatDateTime } from '$lib/utils/formatters';
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
	import { ArrowRight } from '@lucide/svelte';
	import type {
		SessionActionEvent,
		SessionLogEvent,
		SessionReplayEvent
	} from '$lib/types/exceptions';

	type Session = {
		id: string;
		startedAt: string;
		endedAt?: string | null;
		duration: number;
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
	let loading = $state(true);
	let error = $state('');
	let notFound = $state(false);

	let currentTimeMs = $state(0);
	let replayRef = $state<{ seek: (ms: number) => void } | null>(null);

	function handleSeek(offsetMs: number) {
		replayRef?.seek(offsetMs);
	}

	const hasLogs = $derived(recordingLogs.length > 0);
	const hasActions = $derived(recordingActions.length > 0);
	const defaultTab = $derived(hasLogs ? 'logs' : 'actions');

	const ABANDONED_AFTER_MS = 15 * 60_000;

	function durationLabel(s: Session | null): string {
		if (!s) return '—';
		if (s.endedAt) return formatDuration(s.duration);
		const startedMs = Date.parse(s.startedAt);
		if (Number.isFinite(startedMs) && Date.now() - startedMs >= ABANDONED_AFTER_MS) {
			return 'Abandoned';
		}
		return 'in progress';
	}

	async function loadAll() {
		loading = true;
		error = '';
		notFound = false;

		try {
			const startedAt = pageState.url.searchParams.get('t');
			const detail = await api.post(`/sessions/${sessionId}`, startedAt ? { startedAt } : {}, {
				projectId: projectsState.currentProjectId ?? undefined
			});
			session = detail.session;
			exceptions = detail.exceptions || [];

			const recording = await api.get(`/sessions/${sessionId}/recording`, {
				projectId: projectsState.currentProjectId ?? undefined
			});
			recordingEvents = recording?.events ?? [];
			recordingLogs = recording?.logs ?? [];
			recordingActions = recording?.actions ?? [];
		} catch (e) {
			if (getErrorStatus(e) === 404) {
				notFound = true;
			} else {
				error = getErrorMessage(e) || 'Failed to load session';
			}
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		loadAll();
	});
</script>

<div class="space-y-6">
	<PageHeader
		title="Session"
		onBack={createSmartBackHandler({ fallbackPath: resolve('/sessions') })}
	/>

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
			<Card.Content class="grid grid-cols-2 gap-4 sm:grid-cols-3">
				<LabelValue label="Started" value={formatDateTime(session.startedAt, { timezone })} />
				<LabelValue label="Duration" value={durationLabel(session)} />
				<LabelValue label="App version" value={session.appVersion || '—'} />
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

		{#if recordingEvents && recordingEvents.length > 0}
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
						Console output and recorded actions across the session. Times are offsets from session
						start.
					</Card.Description>
				</Card.Header>
				<Card.Content>
					<Tabs.Root value={defaultTab}>
						<Tabs.List class="mb-4">
							{#if hasLogs}
								<Tabs.Trigger value="logs">Logs ({recordingLogs.length})</Tabs.Trigger>
							{/if}
							{#if hasActions}
								<Tabs.Trigger value="actions">Actions ({recordingActions.length})</Tabs.Trigger>
							{/if}
						</Tabs.List>
						{#if hasLogs}
							<Tabs.Content value="logs">
								<SessionLogsTable
									logs={recordingLogs}
									startedAt={session.startedAt}
									{currentTimeMs}
									onSeek={handleSeek}
								/>
							</Tabs.Content>
						{/if}
						{#if hasActions}
							<Tabs.Content value="actions">
								<SessionActionsTable
									actions={recordingActions}
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
