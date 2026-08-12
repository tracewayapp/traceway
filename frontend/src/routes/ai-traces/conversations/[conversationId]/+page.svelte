<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api';
	import { formatDuration, formatDateTime } from '$lib/utils/formatters';
	import { getTimezone } from '$lib/state/timezone.svelte';
	import * as Card from '$lib/components/ui/card';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { ErrorDisplay } from '$lib/components/ui/error-display';
	import { projectsState } from '$lib/state/projects.svelte';
	import PageHeader from '$lib/components/issues/page-header.svelte';
	import { createSmartBackHandler } from '$lib/utils/back-navigation';
	import { resolve } from '$app/paths';
	import { formatCost, formatTokens, formatCount } from '$lib/utils/ai-format';
	import {
		buildConversationTimeline,
		extractMessages,
		formatConversationContent,
		type ConversationTurn,
		type TimelineEntry
	} from '$lib/utils/ai-conversation';
	import ConversationMessages from '$lib/components/ai/conversation-messages.svelte';
	import FlaggedBadge from '$lib/components/ai/flagged-badge.svelte';
	import { addStickyParamsToHref } from '$lib/utils/navigation';

	type ApiTurn = ConversationTurn & {
		userId: string;
		provider: string;
		inputTokens: number;
		outputTokens: number;
		toolCallCount: number;
		toolNames: string[] | null;
		flagged: boolean;
		flaggedTerms: string[] | null;
	};

	type ConversationStats = {
		turns: number;
		totalTokens: number;
		totalCost: number;
		toolCallCount: number;
		avgDuration: number;
		models: string[] | null;
		flagged: boolean;
		flaggedTerms: string[] | null;
		userId: string;
		firstSeen: string;
		lastSeen: string;
	};

	type ConversationDetailResponse = {
		data: ApiTurn[];
		stats: ConversationStats | null;
	};

	let { data } = $props();

	const timezone = $derived(getTimezone());

	let response = $state<ConversationDetailResponse | null>(null);
	let loading = $state(true);
	let error = $state('');
	let notFound = $state(false);
	let showRawJson = $state(false);

	const conversationId = $derived(decodeURIComponent(data.conversationId));

	const turns = $derived(response?.data ?? []);
	const stats = $derived(response?.stats ?? null);

	// One merged chat timeline when history prefix-matching works; falls back
	// to per-turn sections otherwise.
	const timeline = $derived<TimelineEntry[] | null>(
		turns.length > 0 ? buildConversationTimeline(turns) : null
	);

	function turnHref(turn: ApiTurn): string {
		const base = resolve('/ai-traces/[traceName]/[traceId]', {
			traceName: encodeURIComponent(turn.traceName),
			traceId: turn.id
		});
		return addStickyParamsToHref(`${base}?t=${encodeURIComponent(turn.recordedAt)}`, 'preset', 'from', 'to');
	}

	function conversationDurationLabel(stats: ConversationStats): string {
		const from = new Date(stats.firstSeen).getTime();
		const to = new Date(stats.lastSeen).getTime();
		const spanMs = Math.max(0, to - from);
		if (spanMs < 1000) return '< 1s';
		if (spanMs < 60_000) return `${Math.round(spanMs / 1000)}s`;
		if (spanMs < 3_600_000) return `${Math.round(spanMs / 60_000)}m`;
		return `${(spanMs / 3_600_000).toFixed(1)}h`;
	}

	async function loadData() {
		loading = true;
		error = '';
		notFound = false;

		try {
			const result = await api.post(
				'/ai-conversations/conversation',
				{ conversationId },
				{ projectId: projectsState.currentProjectId ?? undefined }
			);
			response = result;
		} catch (e: unknown) {
			console.error(e);
			const err = e as { status?: number; message?: string };
			if (err.status === 404) {
				notFound = true;
			} else {
				error = err.message || 'Failed to load conversation';
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
	<PageHeader
		title="Conversation"
		subtitle={conversationId}
		onBack={createSmartBackHandler({
			fallbackPath: resolve('/ai-traces/conversations')
		})}
	/>

	{#if loading}
		<div class="flex items-center justify-center py-20">
			<LoadingCircle size="xlg" />
		</div>
	{:else if notFound}
		<ErrorDisplay
			status={404}
			title="Conversation Not Found"
			description="The conversation you're looking for doesn't exist or may have expired."
			onBack={createSmartBackHandler({ fallbackPath: resolve('/ai-traces/conversations') })}
			backLabel="Back to Conversations"
			onRetry={loadData}
			identifier={conversationId}
		/>
	{:else if error}
		<ErrorDisplay
			status={400}
			title="Failed to Load Conversation"
			description={error}
			onBack={createSmartBackHandler({ fallbackPath: resolve('/ai-traces/conversations') })}
			backLabel="Back to Conversations"
			onRetry={loadData}
		/>
	{:else if stats}
		<div class="grid grid-cols-2 gap-4 md:grid-cols-3 lg:grid-cols-6">
			<div>
				<p class="text-sm text-muted-foreground">Turns</p>
				<p class="text-2xl font-semibold tabular-nums">{formatCount(stats.turns)}</p>
			</div>
			<div>
				<p class="text-sm text-muted-foreground">Total cost</p>
				<p class="text-2xl font-semibold tabular-nums">{formatCost(stats.totalCost)}</p>
			</div>
			<div>
				<p class="text-sm text-muted-foreground">Total tokens</p>
				<p class="text-2xl font-semibold tabular-nums">{formatTokens(stats.totalTokens)}</p>
			</div>
			<div>
				<p class="text-sm text-muted-foreground">Tool calls</p>
				<p class="text-2xl font-semibold tabular-nums">{formatCount(stats.toolCallCount)}</p>
			</div>
			<div class="min-w-0">
				<p class="text-sm text-muted-foreground">User</p>
				{#if stats.userId}
					<a
						class="block truncate font-mono text-sm text-primary hover:underline"
						href={addStickyParamsToHref(
							`${resolve('/ai-traces/conversations')}?userId=${encodeURIComponent(stats.userId)}`,
							'preset',
							'from',
							'to'
						)}
						title={stats.userId}
					>
						{stats.userId}
					</a>
				{:else}
					<p class="text-sm text-muted-foreground">-</p>
				{/if}
			</div>
			<div>
				<p class="text-sm text-muted-foreground">Duration</p>
				<p class="text-2xl font-semibold tabular-nums">{conversationDurationLabel(stats)}</p>
			</div>
		</div>

		<Card.Root>
			<Card.Header class="flex flex-row items-center justify-between">
				<div>
					<div class="flex items-center gap-2">
						<Card.Title>Conversation</Card.Title>
						{#if stats.flagged && stats.flaggedTerms?.length}
							<FlaggedBadge terms={stats.flaggedTerms} />
						{/if}
					</div>
					<Card.Description>
						{formatDateTime(stats.firstSeen, { timezone })} to {formatDateTime(stats.lastSeen, { timezone })}
					</Card.Description>
				</div>
				<button
					class="text-xs text-muted-foreground transition-colors hover:text-foreground"
					onclick={() => (showRawJson = !showRawJson)}
				>
					{showRawJson ? 'Chat view' : 'Raw JSON'}
				</button>
			</Card.Header>
			<Card.Content>
				{#if showRawJson}
					<div class="space-y-6">
						{#each turns as turn, i}
							<div>
								<p class="mb-2 text-xs font-medium tracking-wide text-muted-foreground uppercase">
									Turn {i + 1} · {formatDateTime(turn.recordedAt, { timezone })}
								</p>
								{#if turn.input}
									<div class="mb-2 max-h-96 overflow-auto rounded-md bg-muted p-4">
										<pre class="font-mono text-sm break-words whitespace-pre-wrap">{formatConversationContent(turn.input)}</pre>
									</div>
								{/if}
								{#if turn.output}
									<div class="max-h-96 overflow-auto rounded-md bg-muted p-4">
										<pre class="font-mono text-sm break-words whitespace-pre-wrap">{formatConversationContent(turn.output)}</pre>
									</div>
								{/if}
							</div>
						{/each}
					</div>
				{:else if timeline}
					<div class="space-y-3">
						{#each timeline as entry, i}
							{#if entry.kind === 'message'}
								<ConversationMessages messages={[entry.message]} />
							{:else}
								{@const turnNumber = timeline.slice(0, i + 1).filter((e) => e.kind === 'turn-meta').length}
								<div class="flex items-center gap-2 py-1 text-xs text-muted-foreground">
									<div class="h-px flex-1 bg-border"></div>
									<span class="whitespace-nowrap">
										Turn {turnNumber}
										{#if entry.turn.model}
											· <span class="font-mono">{entry.turn.model}</span>
										{/if}
										· {formatCost(entry.turn.totalCost)}
										· {formatTokens(entry.turn.totalTokens)} tokens
										· {formatDuration(entry.turn.duration)}
										·
										<a
											class="text-primary hover:underline"
											href={turnHref(entry.turn as ApiTurn)}
											onclick={(e) => e.stopPropagation()}
										>
											View call →
										</a>
									</span>
									<div class="h-px flex-1 bg-border"></div>
								</div>
							{/if}
						{/each}
					</div>
				{:else}
					<!-- Fallback: history prefix-matching failed; render each turn's own
					     parsed conversation as a separate section. -->
					<div class="space-y-6">
						{#each turns as turn, i}
							{@const messages = extractMessages(turn.input, turn.output)}
							<div class="rounded-md border p-4">
								<div class="mb-3 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-muted-foreground">
									<span class="font-medium">Turn {i + 1}</span>
									{#if turn.model}
										· <span class="font-mono">{turn.model}</span>
									{/if}
									· {formatCost(turn.totalCost)}
									· {formatTokens(turn.totalTokens)} tokens
									· {formatDuration(turn.duration)}
									· {formatDateTime(turn.recordedAt, { timezone })}
									·
									<a class="text-primary hover:underline" href={turnHref(turn)}>View call →</a>
								</div>
								{#if messages}
									<ConversationMessages {messages} />
								{:else}
									<p class="text-sm text-muted-foreground">
										No conversation payload was captured for this turn.
									</p>
								{/if}
							</div>
						{/each}
					</div>
				{/if}
			</Card.Content>
		</Card.Root>
	{/if}
</div>
