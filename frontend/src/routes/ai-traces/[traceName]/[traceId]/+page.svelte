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
	import PageHeader from '$lib/components/issues/page-header.svelte';
	import { createSmartBackHandler } from '$lib/utils/back-navigation';
	import { resolve } from '$app/paths';
	import TraceLogsPanel from '$lib/components/trace-logs/trace-logs-panel.svelte';
	import { traceIdUuidToHex } from '$lib/utils/span-id';

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

	function formatCost(cost: number): string {
		if (cost === 0) return '$0';
		if (cost < 0.001) return `$${cost.toFixed(6)}`;
		if (cost < 0.01) return `$${cost.toFixed(4)}`;
		if (cost < 1) return `$${cost.toFixed(3)}`;
		return `$${cost.toFixed(2)}`;
	}

	type ChatMessage = {
		role: string;
		content: string | ContentPart[];
	};

	type ContentPart = {
		type: string;
		text?: string;
		image_url?: { url: string };
	};

	function tryParseJson(str: string): any {
		try {
			return JSON.parse(str);
		} catch {
			return null;
		}
	}

	function extractMessages(input: string, output: string): ChatMessage[] | null {
		const inputParsed = tryParseJson(input);
		if (!inputParsed) return null;

		const messages: ChatMessage[] = [];

		// Extract input messages — could be {messages: [...]} or just [...]
		const inputMessages =
			inputParsed?.messages ?? (Array.isArray(inputParsed) ? inputParsed : null);
		if (!Array.isArray(inputMessages)) return null;

		for (const msg of inputMessages) {
			if (msg.role && msg.content !== undefined) {
				messages.push({ role: msg.role, content: msg.content });
			}
		}

		// Extract assistant response from output
		const outputParsed = tryParseJson(output);
		if (outputParsed) {
			// OpenAI-style: {choices: [{message: {role, content}}]}
			const choices = outputParsed?.choices;
			if (Array.isArray(choices) && choices.length > 0) {
				const choice = choices[0];
				const msg = choice?.message;
				if (msg?.content) {
					messages.push({ role: msg.role || 'assistant', content: msg.content });
				}
			}
			// Anthropic-style: {content: [{type: "text", text: "..."}]}
			else if (Array.isArray(outputParsed?.content)) {
				const text = outputParsed.content
					.filter((c: any) => c.type === 'text')
					.map((c: any) => c.text)
					.join('');
				if (text) {
					messages.push({ role: outputParsed.role || 'assistant', content: text });
				}
			}
		}

		return messages.length > 0 ? messages : null;
	}

	function getMessageText(content: string | ContentPart[]): string {
		if (typeof content === 'string') return content;
		if (Array.isArray(content)) {
			return content
				.filter((p) => p.type === 'text' && p.text)
				.map((p) => p.text!)
				.join('\n');
		}
		return '';
	}

	function getMessageImages(content: string | ContentPart[]): string[] {
		if (!Array.isArray(content)) return [];
		return content
			.filter((p) => p.type === 'image_url' && p.image_url?.url)
			.map((p) => p.image_url!.url);
	}

	function getRoleLabel(role: string): string {
		switch (role) {
			case 'system':
				return 'System';
			case 'user':
				return 'User';
			case 'assistant':
				return 'Assistant';
			case 'tool':
				return 'Tool';
			case 'function':
				return 'Function';
			default:
				return role;
		}
	}

	function formatConversationContent(raw: string): string {
		const parsed = tryParseJson(raw);
		if (parsed) {
			return JSON.stringify(parsed, null, 2);
		}
		return raw;
	}

	let showRawJson = $state(false);

	async function loadData() {
		loading = true;
		error = '';
		notFound = false;

		try {
			const result = await api.post(
				`/ai-traces/${data.traceId}`,
				{},
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
				<Card.Title>Trace Details</Card.Title>
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
						<div class="space-y-3">
							{#each chatMessages as msg}
								<div class="flex gap-3 {msg.role === 'assistant' ? '' : ''}">
									<div
										class="flex h-7 w-7 shrink-0 items-center justify-center rounded-full text-xs font-medium
										{msg.role === 'user'
											? 'bg-primary text-primary-foreground'
											: msg.role === 'assistant'
												? 'border bg-muted text-muted-foreground'
												: msg.role === 'system'
													? 'bg-amber-500/15 text-amber-700 dark:text-amber-400'
													: 'bg-muted text-muted-foreground'}"
									>
										{getRoleLabel(msg.role).charAt(0)}
									</div>
									<div class="min-w-0 flex-1">
										<p class="mb-1 text-xs font-medium text-muted-foreground">
											{getRoleLabel(msg.role)}
										</p>
										{#each getMessageImages(msg.content) as imageUrl}
											{#if imageUrl && !imageUrl.includes('REDACTED')}
												<div class="mb-2 max-w-sm">
													<img
														src={imageUrl}
														alt="Attached image"
														class="max-h-64 rounded-md border object-contain"
													/>
												</div>
											{/if}
										{/each}
										{#if getMessageText(msg.content)}
											<div
												class="rounded-lg px-3 py-2 text-sm break-words whitespace-pre-wrap
												{msg.role === 'user'
													? 'bg-primary/10'
													: msg.role === 'system'
														? 'bg-amber-500/10 font-mono text-xs'
														: 'bg-muted'}"
											>
												{getMessageText(msg.content)}
											</div>
										{/if}
									</div>
								</div>
							{/each}
						</div>
					{/if}
				</Card.Content>
			</Card.Root>
		{/if}

		<TraceLogsPanel
			projectId={projectsState.currentProjectId ?? ''}
			traceId={traceIdUuidToHex(trace.id)}
			distributedTraceId={trace.distributedTraceId ?? null}
			spans={[]}
			rootSpan={{ id: trace.id, name: trace.traceName }}
			traceRecordedAt={trace.recordedAt}
		/>
	{/if}
</div>
