<script lang="ts">
	import * as Popover from '$lib/components/ui/popover';
	import { resolve } from '$app/paths';
	import { api } from '$lib/api';
	import { projectsState } from '$lib/state/projects.svelte';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { formatMetricLabel } from '$lib/utils/metric-format';
	import { Film } from '@lucide/svelte';

	let {
		metric,
		unit,
		fromDateUTC,
		toDateUTC,
		tagFilters = {}
	} = $props<{
		metric: string;
		unit: string;
		fromDateUTC: string;
		toDateUTC: string;
		tagFilters?: Record<string, string>;
	}>();

	type Exemplar = {
		recordedAt: string;
		value: number;
		sessionId: string;
		dims: Record<string, string>;
		/** The element behind the value, when the browser could name it. */
		target?: string;
	};

	let open = $state(false);
	let loading = $state(false);
	let failed = $state(false);
	let exemplars = $state<Exemplar[]>([]);

	async function load() {
		loading = true;
		failed = false;
		try {
			const response: { exemplars?: Exemplar[] } = await api.post(
				'/metrics/exemplars',
				{ name: metric, from: fromDateUTC, to: toDateUTC, tagFilters },
				{ projectId: projectsState.currentProjectId ?? undefined }
			);
			exemplars = response.exemplars ?? [];
		} catch {
			failed = true;
		} finally {
			loading = false;
		}
	}
</script>

<Popover.Root
	bind:open
	onOpenChange={(value) => {
		if (value) load();
	}}
>
	<Popover.Trigger
		class="flex items-center gap-1 rounded px-1.5 py-0.5 text-xs text-muted-foreground hover:bg-muted hover:text-foreground"
		title="Sessions behind the worst values in this range"
	>
		<Film class="h-3 w-3" />
		Slow sessions
	</Popover.Trigger>
	<Popover.Content class="w-80 p-0" align="end">
		<div class="border-b px-3 py-2 text-xs font-medium">Worst values of {metric}</div>
		{#if loading}
			<div class="flex justify-center p-4"><LoadingCircle size="md" /></div>
		{:else if failed}
			<div class="p-3 text-xs text-destructive">The sessions could not be loaded.</div>
		{:else if exemplars.length === 0}
			<div class="p-3 text-xs text-muted-foreground">
				No session was kept for this range. Sessions are kept for values rated poor.
			</div>
		{:else}
			<ul class="max-h-64 overflow-auto py-1">
				{#each exemplars as exemplar (exemplar.sessionId + exemplar.recordedAt)}
					<li>
						<a
							href={resolve(`/sessions/${exemplar.sessionId}`)}
							class="flex items-center justify-between gap-3 px-3 py-1.5 text-xs hover:bg-muted"
						>
							<span class="min-w-0 text-muted-foreground">
								<span class="block truncate"
									>{exemplar.dims['url.path'] ?? exemplar.dims['screen'] ?? 'Session'}</span
								>
								{#if exemplar.target}
									<span class="block truncate font-mono text-[11px]" title={exemplar.target}
										>{exemplar.target}</span
									>
								{/if}
							</span>
							<span class="shrink-0 font-medium tabular-nums"
								>{formatMetricLabel(exemplar.value, unit)}</span
							>
						</a>
					</li>
				{/each}
			</ul>
		{/if}
	</Popover.Content>
</Popover.Root>
