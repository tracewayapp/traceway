<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { api } from '$lib/api';
	import * as Card from '$lib/components/ui/card';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { formatDuration, getStatusColor, formatDateTime } from '$lib/utils/formatters';
	import { getTimezone } from '$lib/state/timezone.svelte';
	import { projectsState } from '$lib/state/projects.svelte';
	import { ArrowRight, GitBranch } from 'lucide-svelte';
	import type { DistributedTraceResponse, DistributedTraceNode } from '$lib/types/distributed-trace';

	interface Props {
		distributedTraceId: string;
		currentExceptionHash?: string;
		recordedAt?: string;
	}

	let { distributedTraceId, currentExceptionHash, recordedAt }: Props = $props();

	const timezone = $derived(getTimezone());

	let response = $state<DistributedTraceResponse | null>(null);
	let loading = $state(true);
	let error = $state('');

	async function loadTrace() {
		loading = true;
		error = '';
		try {
			response = (await api.post(
				`/distributed-traces/${distributedTraceId}`,
				recordedAt ? { recordedAt } : {}
			)) as DistributedTraceResponse;
		} catch (e: any) {
			error = e.message || 'Failed to load distributed trace';
		} finally {
			loading = false;
		}
	}

	function navigateToNode(node: DistributedTraceNode) {
		projectsState.selectProject(node.projectId);
		if (node.traceType === 'task' && node.task) {
			goto(`/tasks/${encodeURIComponent(node.task.taskName)}/${node.task.id}?preset=24h`);
		} else if (node.traceType === 'ai_trace' && node.aiTrace) {
			goto(`/ai-traces/${encodeURIComponent(node.aiTrace.traceName)}/${node.aiTrace.id}?preset=24h`);
		} else if (node.traceType === 'exception' && node.exception) {
			goto(`/issues/${node.exception.exceptionHash}?preset=24h`);
		} else if (node.endpoint) {
			goto(`/endpoints/${encodeURIComponent(node.endpoint.endpoint)}/${node.endpoint.id}?preset=24h`);
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
			<Card.Description>
				This trace spans across multiple services
			</Card.Description>
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
			<Card.Description>
				This trace spans across multiple services
			</Card.Description>
		</Card.Header>
		<Card.Content>
			<div class="space-y-3">
				{#each response.nodes as node, i}
					<div class="flex items-center gap-3 rounded-md border p-3 {i > 0 ? 'ml-6' : ''}">
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
								<span class="shrink-0 font-mono text-sm {getStatusColor(node.endpoint.statusCode)}">
									{node.endpoint.statusCode}
								</span>
							{/if}
							{#if node.traceType === 'ai_trace' && node.aiTrace}
								<Badge variant="secondary" class="shrink-0">{node.aiTrace.provider || node.aiTrace.model || 'AI'}</Badge>
							{/if}
							{#if node.traceType !== 'exception'}
								<span class="shrink-0 font-mono text-sm text-muted-foreground">
									{formatDuration(
										node.traceType === 'task' ? node.task?.duration ?? 0
										: node.traceType === 'ai_trace' ? node.aiTrace?.duration ?? 0
										: node.endpoint?.duration ?? 0
									)}
								</span>
							{/if}
							{#if node.exception}
								<Badge variant="destructive" class="shrink-0">Exception</Badge>
							{/if}
						</div>
						{#if currentExceptionHash && node.traceType === 'exception' && node.exception?.exceptionHash === currentExceptionHash}
						<Badge class="bg-blue-500 hover:bg-blue-500 text-white">You're here</Badge>
					{:else}
						<Button variant="ghost" size="sm" onclick={() => navigateToNode(node)}>
							View
							<ArrowRight class="ml-1 h-3 w-3" />
						</Button>
					{/if}
					</div>
					{/each}
			</div>
		</Card.Content>
	</Card.Root>
{/if}
