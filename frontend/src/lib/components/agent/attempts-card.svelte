<script lang="ts">
	import {
		Card,
		CardContent,
		CardDescription,
		CardHeader,
		CardTitle
	} from '$lib/components/ui/card';
	import * as Table from '$lib/components/ui/table';
	import StatusPill from '$lib/components/traceway/status-pill.svelte';
	import TableContainer from '$lib/components/traceway/table-container.svelte';
	import { ExternalLink } from '@lucide/svelte';
	import { api } from '$lib/api';
	import { projectsState } from '$lib/state/projects.svelte';
	import { resolveHref } from '$lib/utils/links';
	import { createRowClickHandler } from '$lib/utils/navigation';
	import { formatCost, statusLabel, statusTone } from '$lib/utils/agent';
	import type { AttemptWithLinks } from '$lib/types/agent';

	interface Props {
		hash: string;
		refreshKey?: number;
	}

	let { hash, refreshKey = 0 }: Props = $props();

	let attempts = $state<AttemptWithLinks[]>([]);
	let loaded = $state(false);

	async function load() {
		try {
			const res = await api.get(`/agent/attempts/by-subject?hash=${encodeURIComponent(hash)}`, {
				projectId: projectsState.currentProjectId ?? undefined
			});
			attempts = res.attempts ?? [];
		} catch {
			attempts = [];
		} finally {
			loaded = true;
		}
	}

	$effect(() => {
		void refreshKey;
		if (hash) load();
	});

	function pullRequest(attempt: AttemptWithLinks) {
		return attempt.links.find((link) => link.kind === 'pr');
	}
</script>

{#if loaded && attempts.length > 0}
	<Card data-testid="attempts-card">
		<CardHeader>
			<CardTitle>Fix attempts</CardTitle>
			<CardDescription>What the agent tried on this issue, newest first</CardDescription>
		</CardHeader>
		<CardContent>
			<TableContainer>
				<Table.Root>
					<Table.Header>
						<Table.Row>
							<Table.Head>#</Table.Head>
							<Table.Head>Status</Table.Head>
							<Table.Head>Agent</Table.Head>
							<Table.Head>Pull request</Table.Head>
							<Table.Head>Cost</Table.Head>
							<Table.Head>Started</Table.Head>
						</Table.Row>
					</Table.Header>
					<Table.Body>
						{#each attempts as attempt (attempt.id)}
							{@const pr = pullRequest(attempt)}
							<Table.Row
								class="cursor-pointer"
								onclick={createRowClickHandler(`/agent/${attempt.id}`)}
							>
								<Table.Cell>{attempt.number}</Table.Cell>
								<Table.Cell>
									<StatusPill
										tone={statusTone(attempt.status)}
										label={statusLabel(attempt.status)}
									/>
								</Table.Cell>
								<Table.Cell class="text-muted-foreground">
									{attempt.agent || '-'}{attempt.model ? ` · ${attempt.model}` : ''}
								</Table.Cell>
								<Table.Cell>
									{#if pr}
										<a
											{...{ href: pr.url }}
											target="_blank"
											rel="noopener noreferrer"
											onclick={(e) => e.stopPropagation()}
											class="inline-flex items-center gap-1 text-blue-600 hover:underline dark:text-blue-400"
										>
											{pr.externalRef}
											<ExternalLink class="h-3 w-3" />
										</a>
									{:else}
										<span class="text-muted-foreground">-</span>
									{/if}
								</Table.Cell>
								<Table.Cell>{formatCost(attempt.costUsd)}</Table.Cell>
								<Table.Cell class="text-muted-foreground">
									{attempt.startedAt ? new Date(attempt.startedAt).toLocaleString() : 'not yet'}
								</Table.Cell>
							</Table.Row>
						{/each}
					</Table.Body>
				</Table.Root>
			</TableContainer>
			<p class="mt-2 text-xs text-muted-foreground">
				<a {...{ href: resolveHref('/agent') }} class="hover:underline">All attempts</a>
			</p>
		</CardContent>
	</Card>
{/if}
