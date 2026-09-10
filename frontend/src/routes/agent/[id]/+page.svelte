<script lang="ts">
	import { untrack } from 'svelte';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import PageHeader from '$lib/components/traceway/page-header.svelte';
	import StatusPill from '$lib/components/traceway/status-pill.svelte';
	import { Card, CardContent, CardHeader, CardTitle } from '$lib/components/ui/card';
	import { Button } from '$lib/components/ui/button';
	import { Badge } from '$lib/components/ui/badge';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { ErrorDisplay } from '$lib/components/ui/error-display';
	import { ErrorAlert } from '$lib/components/ui/error-alert';
	import * as Tabs from '$lib/components/ui/tabs';
	import { Check, ExternalLink, Send, X } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';
	import { api } from '$lib/api';
	import { projectsState } from '$lib/state/projects.svelte';
	import { authState } from '$lib/state/auth.svelte';
	import { agentState } from '$lib/state/agent.svelte';
	import { createSmartBackHandler } from '$lib/utils/back-navigation';
	import { getErrorMessage, getErrorStatus } from '$lib/utils/errors';
	import {
		attemptTitle,
		formatCost,
		isTerminal,
		latestSeq,
		mergeEvents,
		statusLabel,
		statusTone
	} from '$lib/utils/agent';
	import type { AgentMessage, Attempt, AttemptEvent, AttemptLink } from '$lib/types/agent';

	const id = $derived(page.params.id ?? '');
	const projectId = $derived(projectsState.currentProjectId ?? undefined);

	let attempt = $state<Attempt | null>(null);
	let links = $state<AttemptLink[]>([]);
	let events = $state<AttemptEvent[]>([]);
	let messages = $state<AgentMessage[]>([]);
	let loading = $state(true);
	let error = $state('');
	let notFound = $state(false);
	let tab = $state('timeline');
	let reply = $state('');
	let sending = $state(false);
	let acting = $state(false);
	let replyError = $state('');

	const POLL_MS = 2000;

	async function loadAttempt() {
		const requestedId = id;
		const res = await api.get(`/agent/attempts/${id}`, { projectId });
		if (requestedId !== id) return;
		attempt = res.attempt;
		links = res.links ?? [];
	}

	async function pollEvents() {
		const requestedId = id;
		let count: number;
		do {
			const res = await api.get(`/agent/attempts/${id}/events?after=${latestSeq(events)}`, {
				projectId
			});
			if (requestedId !== id) return;
			count = (res.events ?? []).length;
			events = mergeEvents(events, res.events ?? []);
			if (attempt && res.status && res.status !== attempt.status) {
				await loadAttempt();
				agentState.refreshBadge();
			}
		} while (count >= 500);
	}

	async function pollMessages() {
		const requestedId = id;
		let count: number;
		do {
			const after = messages.length === 0 ? 0 : messages[messages.length - 1].id;
			const res = await api.get(`/agent/attempts/${id}/messages?after=${after}`, { projectId });
			if (requestedId !== id) return;
			const incoming: AgentMessage[] = res.messages ?? [];
			count = incoming.length;
			if (incoming.length > 0) {
				messages = [
					...new Map([...messages, ...incoming].map((message) => [message.id, message])).values()
				].sort((a, b) => a.id - b.id);
			}
		} while (count >= 200);
	}

	const BLOBS = ['diff', 'report', 'transcript'] as const;
	type BlobName = (typeof BLOBS)[number];
	let blobs = $state<Record<BlobName, string>>({ diff: '', report: '', transcript: '' });
	let blobsLoadedFor = $state('');

	async function fetchBlob(name: BlobName): Promise<string> {
		const res = await fetch(`/api/agent/attempts/${id}/${name}?projectId=${projectId}`, {
			headers: { Authorization: `Bearer ${authState.token}` }
		});
		return res.ok ? res.text() : '';
	}

	async function loadBlobs() {
		if (!attempt?.reportKey) return;
		const version = `${attempt.id}:${attempt.reportKey}:${attempt.updatedAt}`;
		if (blobsLoadedFor === version) return;
		blobsLoadedFor = version;
		const [diff, report, transcript] = await Promise.all(BLOBS.map(fetchBlob));
		if (blobsLoadedFor !== version) return;
		blobs = { diff, report, transcript };
	}

	async function loadAll() {
		loading = true;
		error = '';
		notFound = false;
		try {
			await loadAttempt();
			await Promise.all([pollEvents(), pollMessages()]);
			await loadBlobs();
		} catch (e) {
			if (getErrorStatus(e) === 404) notFound = true;
			else error = getErrorMessage(e, 'Failed to load the attempt');
		} finally {
			loading = false;
		}
	}

	async function sendReply() {
		const body = reply.trim();
		if (!body) return;
		sending = true;
		replyError = '';
		try {
			await api.post(`/agent/attempts/${id}/messages`, { body }, { projectId });
			reply = '';
			await Promise.all([pollMessages(), pollEvents()]);
		} catch (e) {
			replyError = getErrorMessage(e, 'Failed to send the message');
		} finally {
			sending = false;
		}
	}

	async function act(action: 'cancel' | 'approve') {
		acting = true;
		try {
			await api.post(`/agent/attempts/${id}/${action}`, {}, { projectId });
			toast.success(
				action === 'cancel'
					? 'Successfully cancelled the Attempt'
					: 'Successfully approved the Attempt',
				{ position: 'top-center' }
			);
			await loadAttempt();
			await pollEvents();
			agentState.refreshBadge();
		} catch (e) {
			toast.error(getErrorMessage(e, `Failed to ${action} the attempt`));
		} finally {
			acting = false;
		}
	}

	$effect(() => {
		if (!id || !projectId) return;
		untrack(() => {
			attempt = null;
			links = [];
			events = [];
			messages = [];
			blobs = { diff: '', report: '', transcript: '' };
			blobsLoadedFor = '';
			loadAll();
		});
	});

	$effect(() => {
		if (!attempt || isTerminal(attempt.status)) return;
		const timer = setInterval(() => {
			pollEvents().catch(() => {});
			pollMessages().catch(() => {});
		}, POLL_MS);
		return () => clearInterval(timer);
	});

	$effect(() => {
		if (attempt?.reportKey) loadBlobs();
	});

	const pullRequest = $derived(links.find((l) => l.kind === 'pr'));
	const canWrite = $derived(projectsState.canWriteCurrentProject);
	const canReply = $derived(canWrite && attempt !== null && !isTerminal(attempt.status));

	function eventText(event: AttemptEvent): string {
		const payload = event.payload as Record<string, unknown>;
		switch (event.kind) {
			case 'status':
				return `Status: ${statusLabel(String(payload.status ?? ''))}${payload.executor ? ` on ${payload.executor}` : ''}`;
			case 'created':
				return `Attempt ${payload.number} created${payload.origin ? ` from ${payload.origin}` : ''}`;
			case 'assistant_text':
			case 'question':
			case 'tool_result':
				return String(payload.text ?? '');
			case 'tool_call':
				return `${payload.tool ?? 'tool'} ${payload.input ? JSON.stringify(payload.input) : ''}`;
			case 'usage': {
				const usage = (payload.usage ?? {}) as Record<string, number>;
				return `Usage: ${usage.inputTokens ?? 0} in / ${usage.outputTokens ?? 0} out, ${formatCost(usage.costUsd ?? 0)}, ${usage.turns ?? 0} turns`;
			}
			case 'result':
				return `Result: ${payload.text ?? ''}`;
			case 'message':
				return `${payload.direction === 'in' ? 'Reply' : 'Message'} via ${payload.provider}`;
			case 'reclaimed':
				return `Reclaimed from ${payload.previousExecutor} (was ${payload.previousStatus})`;
			case 'error':
				return `Error: ${payload.text ?? ''}`;
			default:
				return JSON.stringify(payload);
		}
	}

	function eventTone(
		kind: string
	): 'success' | 'danger' | 'warning' | 'info' | 'neutral' | 'violet' {
		switch (kind) {
			case 'error':
				return 'danger';
			case 'question':
				return 'warning';
			case 'status':
			case 'created':
				return 'info';
			case 'tool_call':
			case 'tool_result':
				return 'neutral';
			case 'result':
			case 'usage':
				return 'violet';
			default:
				return 'neutral';
		}
	}

	function originTone(
		provider: string
	): 'success' | 'danger' | 'warning' | 'info' | 'neutral' | 'violet' {
		switch (provider) {
			case 'agent':
				return 'violet';
			case 'slack':
				return 'success';
			case 'github':
				return 'neutral';
			default:
				return 'info';
		}
	}
</script>

<div class="space-y-4">
	{#if loading && !attempt}
		<div class="flex items-center justify-center py-20"><LoadingCircle size="xlg" /></div>
	{:else if notFound}
		<ErrorDisplay
			status={404}
			title="Attempt Not Found"
			description="This attempt does not exist in the current project."
			onBack={createSmartBackHandler({ fallbackPath: resolve('/agent') })}
			backLabel="Back to Agent"
			onRetry={() => loadAll()}
		/>
	{:else if error}
		<ErrorDisplay
			status={400}
			title="Something Went Wrong"
			description={error}
			onBack={createSmartBackHandler({ fallbackPath: resolve('/agent') })}
			backLabel="Back to Agent"
			onRetry={() => loadAll()}
		/>
	{:else if attempt}
		{@const current = attempt}
		<PageHeader
			title="{attemptTitle(current.subjectKind, current.subjectRef)} · attempt {current.number}"
			onBack={createSmartBackHandler({ fallbackPath: resolve('/agent') })}
		>
			{#snippet meta()}
				<div class="flex flex-wrap items-center gap-2 text-sm text-muted-foreground">
					<StatusPill tone={statusTone(current.status)} label={statusLabel(current.status)} dot />
					{#if current.agent}<span>{current.agent}{current.model ? ` · ${current.model}` : ''}</span
						>{/if}
					<span>{formatCost(current.costUsd)}</span>
					{#if current.turns}<span>{current.turns} turns</span>{/if}
					{#if pullRequest}
						<a
							{...{ href: pullRequest.url }}
							target="_blank"
							rel="noopener noreferrer"
							class="inline-flex items-center gap-1 text-blue-600 hover:underline dark:text-blue-400"
						>
							{pullRequest.externalRef}
							<ExternalLink class="h-3 w-3" />
						</a>
					{/if}
					{#if current.subjectKind === 'traceway_exception'}
						<a
							{...{ href: resolve(`/issues/${current.subjectRef}`) }}
							class="text-blue-600 hover:underline dark:text-blue-400">View issue</a
						>
					{/if}
				</div>
			{/snippet}
			{#snippet actions()}
				{#if canWrite && current.status === 'pending_approval'}
					<Button onclick={() => act('approve')} disabled={acting}>
						<Check class="mr-2 h-4 w-4" />
						Approve
					</Button>
				{/if}
				{#if canWrite && !isTerminal(current.status)}
					<Button variant="outline" onclick={() => act('cancel')} disabled={acting}>
						<X class="mr-2 h-4 w-4" />
						Cancel attempt
					</Button>
				{/if}
			{/snippet}
		</PageHeader>

		{#if attempt.error}
			<ErrorAlert error={attempt.error} />
		{/if}

		<div class="grid gap-4 lg:grid-cols-[minmax(0,3fr)_minmax(0,2fr)]">
			<Card>
				<CardHeader>
					<Tabs.Root bind:value={tab}>
						<Tabs.List>
							<Tabs.Trigger value="timeline">Timeline</Tabs.Trigger>
							<Tabs.Trigger value="diff">Diff</Tabs.Trigger>
							<Tabs.Trigger value="report">Report</Tabs.Trigger>
							<Tabs.Trigger value="transcript">Transcript</Tabs.Trigger>
						</Tabs.List>
					</Tabs.Root>
				</CardHeader>
				<CardContent>
					{#if tab === 'timeline'}
						{#if events.length === 0}
							<p class="text-sm text-muted-foreground">Waiting for the first event.</p>
						{:else}
							<ol class="space-y-2" data-testid="timeline">
								{#each events as event (event.seq)}
									<li class="flex items-start gap-3 text-sm">
										<span class="w-8 shrink-0 text-right font-mono text-xs text-muted-foreground">
											{event.seq}
										</span>
										<StatusPill
											tone={eventTone(event.kind)}
											label={event.kind.replaceAll('_', ' ')}
										/>
										<span class="min-w-0 flex-1 break-words whitespace-pre-wrap"
											>{eventText(event)}</span
										>
									</li>
								{/each}
							</ol>
						{/if}
					{:else if tab === 'diff'}
						{#if blobs.diff}
							<pre
								class="max-h-[70vh] overflow-auto rounded-md bg-muted p-3 text-xs">{blobs.diff}</pre>
						{:else}
							<p class="text-sm text-muted-foreground">No diff yet.</p>
						{/if}
					{:else if tab === 'report'}
						{#if blobs.report}
							<pre
								class="max-h-[70vh] overflow-auto rounded-md bg-muted p-3 text-sm whitespace-pre-wrap">{blobs.report}</pre>
						{:else}
							<p class="text-sm text-muted-foreground">No report yet.</p>
						{/if}
					{:else if blobs.transcript}
						<pre
							class="max-h-[70vh] overflow-auto rounded-md bg-muted p-3 text-xs">{blobs.transcript}</pre>
					{:else}
						<p class="text-sm text-muted-foreground">
							No transcript yet. The agent's full session is stored when the run finishes.
						</p>
					{/if}
				</CardContent>
			</Card>

			<Card>
				<CardHeader>
					<CardTitle>Thread</CardTitle>
				</CardHeader>
				<CardContent class="space-y-3">
					{#if messages.length === 0}
						<p class="text-sm text-muted-foreground">
							The agent's questions and findings, and replies from every surface, appear here.
						</p>
					{:else}
						<ol class="space-y-3" data-testid="thread">
							{#each messages as message (message.id)}
								<li class="rounded-md border p-3 text-sm">
									<div class="mb-1 flex items-center gap-2 text-xs text-muted-foreground">
										<StatusPill tone={originTone(message.provider)} label={message.provider} />
										<span>{message.direction === 'in' ? 'reply' : message.kind}</span>
										<span>{new Date(message.createdAt).toLocaleString()}</span>
									</div>
									<p class="break-words whitespace-pre-wrap">{message.body}</p>
								</li>
							{/each}
						</ol>
					{/if}
					{#if canReply}
						<form
							class="space-y-2"
							onsubmit={(e) => {
								e.preventDefault();
								sendReply();
							}}
						>
							<ErrorAlert error={replyError} />
							<textarea
								bind:value={reply}
								rows="3"
								placeholder={attempt.status === 'needs_input'
									? 'Answer the agent to continue.'
									: 'Send the agent a message.'}
								class="w-full rounded-md border bg-background p-2 text-sm"
								data-testid="reply-box"
							></textarea>
							<Button type="submit" disabled={sending || !reply.trim()}>
								<Send class="mr-2 h-4 w-4" />
								{sending ? 'Sending...' : 'Send'}
							</Button>
						</form>
					{/if}
					{#if messages.length > 0}
						<Badge variant="outline" class="text-xs">
							{messages.length}
							{messages.length === 1 ? 'message' : 'messages'}
						</Badge>
					{/if}
				</CardContent>
			</Card>
		</div>
	{/if}
</div>
