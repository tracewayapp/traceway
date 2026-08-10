<script lang="ts" module>
	import { api } from '$lib/api';
	import type { ProjectOncall } from '$lib/state/oncall.svelte';

	interface CacheEntry {
		promise: Promise<ProjectOncall | null>;
		fetchedAt: number;
	}

	const cache = new Map<string, CacheEntry>();
	const CACHE_TTL_MS = 60_000;

	function fetchOncall(projectId: string): Promise<ProjectOncall | null> {
		const entry = cache.get(projectId);
		if (entry && Date.now() - entry.fetchedAt < CACHE_TTL_MS) {
			return entry.promise;
		}
		const promise = api
			.get('/oncall/current', { projectId })
			.then((res) => res as ProjectOncall)
			.catch(() => {
				if (cache.get(projectId)?.promise === promise) {
					cache.delete(projectId);
				}
				return null;
			});
		cache.set(projectId, { promise, fetchedAt: Date.now() });
		return promise;
	}
</script>

<script lang="ts">
	import { PhoneCall } from '@lucide/svelte';
	import { projectsState } from '$lib/state/projects.svelte';

	let oncall = $state<ProjectOncall | null>(null);

	$effect(() => {
		const projectId = projectsState.currentProjectId;
		if (!projectId) return;
		fetchOncall(projectId).then((res) => {
			if (projectId === projectsState.currentProjectId) {
				oncall = res;
			}
		});
	});

	const names = $derived(
		(oncall?.oncall ?? []).map((u) => u.name || u.email).join(', ')
	);
</script>

{#if oncall?.team}
	<div
		class="inline-flex items-center gap-1.5 rounded-md border bg-muted/50 px-2 py-1 text-xs text-muted-foreground"
		title={names ? `On call for ${oncall.team.name}: ${names}` : oncall.team.name}
	>
		<PhoneCall class="h-3.5 w-3.5" />
		<span class="whitespace-nowrap">
			On-call: <span class="font-medium text-foreground">{oncall.team.name}</span>{#if names}
				<span>{` — ${names}`}</span>{/if}
		</span>
	</div>
{/if}
