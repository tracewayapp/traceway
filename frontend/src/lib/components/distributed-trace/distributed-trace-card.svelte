<script lang="ts">
	import { onMount } from 'svelte';
	import { SvelteSet } from 'svelte/reactivity';
	import { gotoHref } from '$lib/utils/navigation';
	import { api } from '$lib/api';
	import * as Card from '$lib/components/ui/card';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import SpanWaterfall from '$lib/components/spans/span-waterfall.svelte';
	import { formatDuration, getStatusColor } from '$lib/utils/formatters';
	import { ArrowRight, ChevronDown, ChevronRight, GitBranch } from '@lucide/svelte';
	import type {
		DistributedTraceResponse,
		DistributedTraceNode
	} from '$lib/types/distributed-trace';

	interface Props {
		distributedTraceId: string;
		currentExceptionHash?: string;
		currentNodeId?: string;
		recordedAt?: string;
	}

	let { distributedTraceId, currentExceptionHash, currentNodeId, recordedAt }: Props = $props();

	function isCurrentNode(node: DistributedTraceNode): boolean {
		if (currentExceptionHash && node.traceType === 'exception') {
			return node.exception?.exceptionHash === currentExceptionHash;
		}
		if (currentNodeId) {
			return (
				node.endpoint?.id === currentNodeId ||
				node.task?.id === currentNodeId ||
				node.aiTrace?.id === currentNodeId
			);
		}
		return false;
	}

	let response = $state<DistributedTraceResponse | null>(null);
	let loading = $state(true);
	const expandedSpans = new SvelteSet<string>();

	async function loadTrace() {
		loading = true;
		try {
			response = (await api.post(
				`/distributed-traces/${distributedTraceId}`,
				recordedAt ? { recordedAt } : {}
			)) as DistributedTraceResponse;
		} catch {
			response = null;
		} finally {
			loading = false;
		}
	}

	function nodeKey(node: DistributedTraceNode, index: number): string {
		return node.endpoint?.id ?? node.task?.id ?? node.aiTrace?.id ?? `exception-${index}`;
	}

	function nodeDuration(node: DistributedTraceNode): number {
		return node.endpoint?.duration ?? node.task?.duration ?? node.aiTrace?.duration ?? 0;
	}

	function nodeStartTime(node: DistributedTraceNode): string {
		return node.endpoint?.recordedAt ?? node.task?.recordedAt ?? node.aiTrace?.recordedAt ?? '';
	}

	function toggleSpans(key: string) {
		if (expandedSpans.has(key)) {
			expandedSpans.delete(key);
		} else {
			expandedSpans.add(key);
		}
	}

	function navigateToNode(node: DistributedTraceNode) {
		const project = `&projectId=${encodeURIComponent(node.projectId)}`;
		if (node.traceType === 'task' && node.task) {
			gotoHref(
				`/tasks/${encodeURIComponent(node.task.taskName)}/${node.task.id}?preset=24h&t=${encodeURIComponent(node.task.recordedAt)}${project}`
			);
		} else if (node.traceType === 'ai_trace' && node.aiTrace) {
			gotoHref(
				`/ai-traces/${encodeURIComponent(node.aiTrace.traceName)}/${node.aiTrace.id}?preset=24h&t=${encodeURIComponent(node.aiTrace.recordedAt)}${project}`
			);
		} else if (node.traceType === 'exception' && node.exception) {
			gotoHref(`/issues/${node.exception.exceptionHash}?preset=24h${project}`);
		} else if (node.endpoint) {
			gotoHref(
				`/endpoints/${encodeURIComponent(node.endpoint.endpoint)}/${node.endpoint.id}?preset=24h&t=${encodeURIComponent(node.endpoint.recordedAt)}${project}`
			);
		}
	}

	onMount(() => {
		loadTrace();
	});
</script>

{#if loading}
	<Card.Root>
		<Card.Header>
			<div class="flex items-center gap-2">
				<GitBranch class="h-5 w-5 text-muted-foreground" />
				<Card.Title>Distributed Trace</Card.Title>
			</div>
			<Card.Description>This trace spans across multiple services</Card.Description>
		</Card.Header>
		<Card.Content>
			<div class="flex items-center justify-center py-6">
				<LoadingCircle size="md" />
			</div>
		</Card.Content>
	</Card.Root>
{:else if response && response.nodes.length > 1}
	<Card.Root>
		<Card.Header>
			<div class="flex items-center gap-2">
				<GitBranch class="h-5 w-5 text-muted-foreground" />
				<Card.Title>Distributed Trace</Card.Title>
			</div>
			<Card.Description>This trace spans across multiple services</Card.Description>
		</Card.Header>
		<Card.Content>
			<div class="space-y-3">
				{#each response.nodes as node, i (nodeKey(node, i))}
					{@const key = nodeKey(node, i)}
					{@const spanCount = node.spans?.length ?? 0}
					<div class={i > 0 ? 'ml-6' : ''}>
						<div class="flex items-center gap-3 rounded-md border p-3">
							<div class="flex min-w-0 flex-1 items-center gap-3">
								<Badge variant="outline" class="shrink-0">{node.projectName}</Badge>
								<span class="truncate font-mono text-sm">
									{#if node.traceType === 'task'}
										{node.task?.taskName}
									{:else if node.traceType === 'ai_trace'}
										{node.aiTrace?.traceName}
									{:else if node.traceType === 'exception'}
										{node.exception?.stackTrace.split('\n')[0]}
									{:else}
										{node.endpoint?.endpoint}
									{/if}
								</span>
								{#if node.traceType === 'endpoint' && node.endpoint}
									<span
										class="shrink-0 font-mono text-sm {getStatusColor(node.endpoint.statusCode)}"
									>
										{node.endpoint.statusCode}
									</span>
								{/if}
								{#if node.traceType === 'ai_trace' && node.aiTrace}
									<Badge variant="secondary" class="shrink-0"
										>{node.aiTrace.provider || node.aiTrace.model || 'AI'}</Badge
									>
								{/if}
								{#if node.traceType !== 'exception'}
									<span class="shrink-0 font-mono text-sm text-muted-foreground">
										{formatDuration(nodeDuration(node))}
									</span>
								{/if}
								{#if node.exception}
									<Badge variant="destructive" class="shrink-0">Exception</Badge>
								{/if}
							</div>
							{#if spanCount > 0}
								<Button
									variant="ghost"
									size="sm"
									class="shrink-0 text-muted-foreground"
									onclick={() => toggleSpans(key)}
									aria-expanded={expandedSpans.has(key)}
								>
									{#if expandedSpans.has(key)}
										<ChevronDown class="mr-1 h-3 w-3" />
									{:else}
										<ChevronRight class="mr-1 h-3 w-3" />
									{/if}
									{spanCount}
									{spanCount === 1 ? 'span' : 'spans'}
								</Button>
							{/if}
							{#if isCurrentNode(node)}
								<Badge class="bg-blue-500 text-white hover:bg-blue-500">You're here</Badge>
							{:else}
								<Button variant="ghost" size="sm" onclick={() => navigateToNode(node)}>
									View
									<ArrowRight class="ml-1 h-3 w-3" />
								</Button>
							{/if}
						</div>
						{#if spanCount > 0 && expandedSpans.has(key)}
							<div class="mt-2 rounded-md border p-2">
								<SpanWaterfall
									spans={node.spans}
									traceDuration={nodeDuration(node)}
									traceStartTime={nodeStartTime(node)}
									rootSpanId={node.endpoint?.spanId ?? node.task?.spanId}
								/>
							</div>
						{/if}
					</div>
				{/each}
			</div>
		</Card.Content>
	</Card.Root>
{/if}
