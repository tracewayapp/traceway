<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api';
	import { formatDuration, formatDateTime } from '$lib/utils/formatters';
	import { getTimezone } from '$lib/state/timezone.svelte';
	import * as Card from '$lib/components/ui/card';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { ErrorDisplay } from '$lib/components/ui/error-display';
	import { projectsState } from '$lib/state/projects.svelte';
	import { LabelValue } from '$lib/components/ui/label-value';
	import { AttributesGrid } from '$lib/components/ui/attributes-grid/index.js';
	import PageHeader from '$lib/components/traceway/page-header.svelte';
	import { createSmartBackHandler } from '$lib/utils/back-navigation';
	import { resolve } from '$app/paths';
	import TraceLogsPanel from '$lib/components/trace-logs/trace-logs-panel.svelte';
	import { traceIdUuidToHex } from '$lib/utils/span-id';
	import { formatCost } from '$lib/utils/ai-format';
	import { extractMessages, formatConversationContent } from '$lib/utils/ai-conversation';
	import ConversationMessages from '$lib/components/ai/conversation-messages.svelte';
	import FlaggedBadge from '$lib/components/ai/flagged-badge.svelte';
	import { addStickyParamsToHref } from '$lib/utils/navigation';

	type AiTrace = {
		id: string;
		recordedAt: string;
		duration: number;
		statusCode: number;
		model: string;
		responseModel: string;
		provider: string;
		operation: string;
		inputTokens: number;
		outputTokens: number;
		totalTokens: number;
		cachedTokens: number;
		reasoningTokens: number;
		inputCost: number;
		outputCost: number;
		totalCost: number;
		traceName: string;
		userId: string;
		finishReason: string;
		serverName: string;
		appVersion: string;
		attributes: Record<string, string> | null;
		distributedTraceId?: string;
		conversationId?: string;
		toolCallCount?: number;
		toolNames?: string[] | null;
		flagged?: boolean;
		flaggedTerms?: string[] | null;
	};

	type AiTraceDetailResponse = {
		aiTrace: AiTrace;
		conversation?: {
			input: string;
			output: string;
		};
	};

	let { data } = $props();

	const timezone = $derived(getTimezone());

	let response = $state<AiTraceDetailResponse | null>(null);
	let loading = $state(true);
	let error = $state('');
	let notFound = $state(false);

	let showRawJson = $state(false);

	async function loadData() {
		loading = true;
		error = '';
		notFound = false;

		try {
			const result = await api.post(
				`/ai-traces/${data.traceId}`,
				data.recordedAt ? { recordedAt: data.recordedAt } : {},
				{ projectId: projectsState.currentProjectId ?? undefined }
			);
			response = result;
		} catch (e: unknown) {
			console.error(e);
			const err = e as { status?: number; message?: string };
			if (err.status === 404) {
				notFound = true;
			} else {
				error = err.message || 'Failed to load AI trace details';
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
		title={decodeURIComponent(data.traceName)}
		subtitle={`Trace ID: ${data.traceId}`}
		onBack={createSmartBackHandler({
			fallbackPath: resolve('/ai-traces/[traceName]', {
				traceName: encodeURIComponent(data.traceName)
			})
		})}
	/>

	{#if loading}
		<div class="flex items-center justify-center py-20">
			<LoadingCircle size="xlg" />
		</div>
	{:else if notFound}
		<ErrorDisplay
			status={404}
			title="AI Trace Not Found"
			description="The AI trace you're looking for doesn't exist or may have expired."
			onBack={createSmartBackHandler({
				fallbackPath: resolve('/ai-traces/[traceName]', {
					traceName: encodeURIComponent(data.traceName)
				})
			})}
			backLabel="Back to Trace"
			onRetry={loadData}
			identifier={data.traceId}
		/>
	{:else if error}
		<ErrorDisplay
			status={400}
			title="Failed to Load AI Trace"
			description={error}
			onBack={createSmartBackHandler({
				fallbackPath: resolve('/ai-traces/[traceName]', {
					traceName: encodeURIComponent(data.traceName)
				})
			})}
			backLabel="Back to Trace"
			onRetry={loadData}
		/>
	{:else if response}
		{@const trace = response.aiTrace}

		<Card.Root>
			<Card.Header>
				<div class="flex items-center gap-2">
					<Card.Title>Trace Details</Card.Title>
					{#if trace.flagged && trace.flaggedTerms?.length}
						<FlaggedBadge terms={trace.flaggedTerms} />
					{/if}
				</div>
				<Card.Description>Details of this specific AI trace</Card.Description>
			</Card.Header>
			<Card.Content>
				<div class="grid grid-cols-2 gap-4 md:grid-cols-4">
					<LabelValue label="Model" value={trace.model || '-'} mono />
					<LabelValue label="Provider" value={trace.provider || '-'} mono />
					<LabelValue label="Duration" value={formatDuration(trace.duration)} mono large />
					<LabelValue
						label="Recorded At"
						value={formatDateTime(trace.recordedAt, { timezone })}
						mono
					/>
					<LabelValue label="Input Tokens" value={trace.inputTokens.toLocaleString()} mono />
					<LabelValue label="Output Tokens" value={trace.outputTokens.toLocaleString()} mono />
					<LabelValue label="Total Tokens" value={trace.totalTokens.toLocaleString()} mono />
					<LabelValue label="Total Cost" value={formatCost(trace.totalCost)} mono large />
					<LabelValue label="Input Cost" value={formatCost(trace.inputCost)} mono />
					<LabelValue label="Output Cost" value={formatCost(trace.outputCost)} mono />
					<LabelValue label="Finish Reason" value={trace.finishReason || '-'} mono />
					<LabelValue label="Operation" value={trace.operation || '-'} mono />
					{#if trace.cachedTokens > 0}
						<LabelValue label="Cached Tokens" value={trace.cachedTokens.toLocaleString()} mono />
					{/if}
					{#if trace.reasoningTokens > 0}
						<LabelValue
							label="Reasoning Tokens"
							value={trace.reasoningTokens.toLocaleString()}
							mono
						/>
					{/if}
					{#if trace.userId}
						<LabelValue label="User ID" value={trace.userId} mono />
					{/if}
					{#if trace.toolCallCount}
						<LabelValue label="Tool Calls" value={trace.toolCallCount.toLocaleString()} mono />
					{/if}
					{#if trace.conversationId}
						<div class="space-y-1">
							<p class="text-sm text-muted-foreground">Conversation</p>
							<a
								class="block truncate font-mono text-sm text-primary hover:underline"
								href={resolve(
									addStickyParamsToHref(
										resolve('/ai-traces/conversations/[conversationId]', {
											conversationId: encodeURIComponent(trace.conversationId)
										}),
										'preset',
										'from',
										'to'
									) as '/'
								)}
								title={trace.conversationId}
							>
								{trace.conversationId}
							</a>
						</div>
					{/if}
					<LabelValue label="Server" value={trace.serverName || '-'} mono />
					<LabelValue label="Version" value={trace.appVersion || '-'} mono />
				</div>
			</Card.Content>
		</Card.Root>

		{#if trace.attributes && Object.keys(trace.attributes).length > 0}
			<Card.Root>
				<Card.Header>
					<Card.Title>Attributes</Card.Title>
					<Card.Description>Additional metadata attached to this trace</Card.Description>
				</Card.Header>
				<Card.Content>
					<AttributesGrid attributes={trace.attributes} />
				</Card.Content>
			</Card.Root>
		{/if}

		{#if response.conversation}
			{@const conv = response.conversation}
			{@const chatMessages = extractMessages(conv.input, conv.output)}

			<Card.Root>
				<Card.Header class="flex flex-row items-center justify-between">
					<div>
						<Card.Title>Conversation</Card.Title>
						<Card.Description>Messages exchanged with the model</Card.Description>
					</div>
					<button
						class="text-xs text-muted-foreground transition-colors hover:text-foreground"
						onclick={() => (showRawJson = !showRawJson)}
					>
						{showRawJson ? 'Chat view' : 'Raw JSON'}
					</button>
				</Card.Header>
				<Card.Content>
					{#if showRawJson || !chatMessages}
						<div class="space-y-4">
							{#if conv.input}
								<div>
									<p class="mb-2 text-xs font-medium tracking-wide text-muted-foreground uppercase">
										Input
									</p>
									<div class="max-h-96 overflow-auto rounded-md bg-muted p-4">
										<pre
											class="font-mono text-sm break-words whitespace-pre-wrap">{formatConversationContent(
												conv.input
											)}</pre>
									</div>
								</div>
							{/if}
							{#if conv.output}
								<div>
									<p class="mb-2 text-xs font-medium tracking-wide text-muted-foreground uppercase">
										Output
									</p>
									<div class="max-h-96 overflow-auto rounded-md bg-muted p-4">
										<pre
											class="font-mono text-sm break-words whitespace-pre-wrap">{formatConversationContent(
												conv.output
											)}</pre>
									</div>
								</div>
							{/if}
						</div>
					{:else}
						<ConversationMessages messages={chatMessages} />
					{/if}
				</Card.Content>
			</Card.Root>
		{/if}

		<TraceLogsPanel
			projectId={projectsState.currentProjectId ?? ''}
			traceId={traceIdUuidToHex(trace.id)}
			distributedTraceId={trace.distributedTraceId ?? null}
			spans={[]}
			traceRecordedAt={trace.recordedAt}
		/>
	{/if}
</div>
