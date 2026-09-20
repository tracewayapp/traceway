<script lang="ts">
	import { onMount } from 'svelte';
	import { gotoHref } from '$lib/utils/navigation';
	import { api } from '$lib/api';
	import * as Card from '$lib/components/ui/card';
	import { Badge } from '$lib/components/ui/badge';
	import SpanGraphNotice from '$lib/components/spans/span-graph-notice.svelte';
	import { Button } from '$lib/components/ui/button';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { formatDuration, getStatusColor } from '$lib/utils/formatters';
	import { ArrowRight, GitBranch, ChevronRight, ChevronDown } from '@lucide/svelte';
	import { SvelteSet } from 'svelte/reactivity';
	import { distributedTraceTree } from '$lib/utils/distributed-trace-tree';
	import type {
		DistributedTraceResponse,
		DistributedTraceNode
	} from '$lib/types/distributed-trace';

	interface Props {
		traceId: string;
		currentExceptionHash?: string;
		currentNodeId?: string;
		recordedAt?: string;
	}

	let { traceId, currentExceptionHash, currentNodeId, recordedAt }: Props = $props();

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

	let response = $state.raw<DistributedTraceResponse | null>(null);
	let loading = $state(true);
	const collapsed = new SvelteSet<string>();
	const treeRows = $derived(distributedTraceTree(response?.nodes ?? [], collapsed));
	function toggle(key: string) {
		if (collapsed.has(key)) collapsed.delete(key);
		else collapsed.add(key);
	}

	async function loadTrace() {
		loading = true;
		try {
			response = (await api.post(
				`/distributed-traces/${traceId}`,
				recordedAt ? { recordedAt } : {}
			)) as DistributedTraceResponse;
		} catch {
			response = null;
		} finally {
			loading = false;
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
				{#each treeRows as row (row.key)}
					{@const node = row.node}
					<div
						class="flex flex-col gap-3 rounded-md border p-3"
						style:margin-left={`${row.depth * 24}px`}
					>
						<div class="flex items-center gap-3">
							{#if row.hasChildren}
								<button
									class="shrink-0 rounded p-1 hover:bg-muted"
									onclick={() => toggle(row.key)}
									aria-label={collapsed.has(row.key) ? 'Expand children' : 'Collapse children'}
									aria-expanded={!collapsed.has(row.key)}
								>
									{#if collapsed.has(row.key)}<ChevronRight class="h-4 w-4" />{:else}<ChevronDown
											class="h-4 w-4"
										/>{/if}
								</button>
							{/if}
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
										{formatDuration(
											node.traceType === 'task'
												? (node.task?.duration ?? 0)
												: node.traceType === 'ai_trace'
													? (node.aiTrace?.duration ?? 0)
													: (node.endpoint?.duration ?? 0)
										)}
									</span>
								{/if}
								{#if node.exception}
									<Badge variant="destructive" class="shrink-0">Exception</Badge>
								{/if}
							</div>
							{#if isCurrentNode(node)}
								<Badge class="bg-blue-500 text-white hover:bg-blue-500">You're here</Badge>
							{:else}
								<Button variant="ghost" size="sm" onclick={() => navigateToNode(node)}>
									View
									<ArrowRight class="ml-1 h-3 w-3" />
								</Button>
							{/if}
						</div>
						<SpanGraphNotice status={node.spanGraphStatus} />
					</div>
				{/each}
			</div>
		</Card.Content>
	</Card.Root>
{/if}
