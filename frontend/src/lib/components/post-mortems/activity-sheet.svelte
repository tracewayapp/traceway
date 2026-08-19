<script lang="ts">
	import { untrack } from 'svelte';
	import * as Sheet from '$lib/components/ui/sheet';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { FilePlus2, Pencil } from '@lucide/svelte';
	import { api } from '$lib/api';
	import { formatDateTime } from '$lib/utils/formatters';
	import { getErrorMessage } from '$lib/utils/errors';
	import { projectsState } from '$lib/state/projects.svelte';

	interface ActivityEvent {
		id: number;
		action: 'created' | 'updated';
		changes: string[];
		userName: string | null;
		createdAt: string;
	}

	interface Props {
		open: boolean;
		postMortemId: string;
	}

	let { open = $bindable(), postMortemId }: Props = $props();

	let events = $state<ActivityEvent[]>([]);
	let loading = $state(false);
	let error = $state('');
	let loadSeq = 0;

	$effect(() => {
		if (!open) return;
		void postMortemId;
		untrack(() => load());
	});

	async function load() {
		const projectId = projectsState.currentProjectId;
		if (projectId === null) return;
		const seq = ++loadSeq;
		loading = true;
		error = '';
		try {
			const res = (await api.get(`/post-mortems/${postMortemId}/activity`, {
				projectId
			})) as { events?: ActivityEvent[] };
			if (seq !== loadSeq) return;
			events = res.events || [];
		} catch (e) {
			if (seq !== loadSeq) return;
			error = e instanceof Error ? getErrorMessage(e) : 'Failed to load activity';
		} finally {
			if (seq === loadSeq) loading = false;
		}
	}

	function describe(event: ActivityEvent): string {
		if (event.action === 'created') return 'created this post-mortem';
		if (event.changes.length > 0) return `edited the ${event.changes.join(', ')}`;
		return 'edited this post-mortem';
	}
</script>

<Sheet.Root bind:open>
	<Sheet.Content side="right" class="flex w-full flex-col gap-0 overflow-y-auto sm:max-w-md">
		<Sheet.Header class="border-b px-6 pb-4">
			<Sheet.Title>Activity</Sheet.Title>
			<Sheet.Description>Every edit made to this post-mortem.</Sheet.Description>
		</Sheet.Header>

		<div class="flex-1 px-6 py-5">
			{#if loading}
				<div class="flex h-32 items-center justify-center">
					<LoadingCircle size="lg" />
				</div>
			{:else if error}
				<p class="text-sm text-red-500">{error}</p>
			{:else if events.length === 0}
				<p class="text-sm text-muted-foreground">No activity recorded yet.</p>
			{:else}
				<div class="space-y-5">
					{#each events as event (event.id)}
						<div class="flex gap-3">
							<div
								class="mt-0.5 flex size-7 shrink-0 items-center justify-center rounded-full bg-muted text-muted-foreground"
							>
								{#if event.action === 'created'}
									<FilePlus2 class="size-3.5" />
								{:else}
									<Pencil class="size-3.5" />
								{/if}
							</div>
							<div class="min-w-0">
								<p class="text-sm">
									<span class="font-medium">{event.userName || 'Someone'}</span>
									{describe(event)}
								</p>
								<p class="text-xs text-muted-foreground">
									{formatDateTime(event.createdAt, { format: 'short' })}
								</p>
							</div>
						</div>
					{/each}
				</div>
			{/if}
		</div>
	</Sheet.Content>
</Sheet.Root>
