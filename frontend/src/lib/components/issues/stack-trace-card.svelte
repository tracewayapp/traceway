<script lang="ts">
	import * as Card from '$lib/components/ui/card';
	import { formatDateTime } from '$lib/utils/formatters';
	import { getTimezone } from '$lib/state/timezone.svelte';
	import Button from '../ui/button/button.svelte';
	import { Archive, ChevronRight, ChevronDown } from 'lucide-svelte';
	import { parseStackTrace } from '$lib/utils/stack-trace-parser';

	interface Props {
		stackTrace: string;
		isMessage?: boolean;
		firstSeen?: string;
		lastSeen?: string;
		totalCount?: number;
		timezone?: string;
		showArchiveDialog: boolean;
		archiving: boolean;
	}

	let {
		stackTrace,
		isMessage = false,
		firstSeen,
		lastSeen,
		totalCount,
		timezone,
		archiving = $bindable(),
		showArchiveDialog = $bindable()
	}: Props = $props();

	const tz = $derived(timezone ?? getTimezone());
	const showStats = $derived(firstSeen && lastSeen && totalCount !== undefined);
	const parsed = $derived(parseStackTrace(stackTrace));

	let expandedGroups = $state<Set<number>>(new Set());

	function toggleGroup(index: number) {
		const next = new Set(expandedGroups);
		if (next.has(index)) {
			next.delete(index);
		} else {
			next.add(index);
		}
		expandedGroups = next;
	}
</script>

<Card.Root>
	<Card.Header class={showStats ? '' : 'gap-0 pb-0'}>
		<div class="flex justify-between">
			<div class="flex items-center gap-2">
				<Card.Title>Stack Trace</Card.Title>
				{#if isMessage}
					<span
						class="inline-flex items-center rounded-md bg-blue-50 px-2 py-1 text-xs font-medium text-blue-700 ring-1 ring-blue-700/10 ring-inset dark:bg-blue-900/30 dark:text-blue-300 dark:ring-blue-400/30"
					>
						Message
					</span>
				{/if}
			</div>
			<Button
				variant="outline"
				size="sm"
				onclick={() => (showArchiveDialog = true)}
				disabled={archiving}
				class="shrink-0 gap-1.5"
			>
				<Archive class="h-4 w-4" />
				Archive
			</Button>
		</div>
		{#if showStats}
			<Card.Description>
				First seen: {formatDateTime(firstSeen!, { timezone: tz })} · Last seen: {formatDateTime(
					lastSeen!,
					{ timezone: tz }
				)} · Total occurrences: {totalCount}
			</Card.Description>
		{/if}
	</Card.Header>
	<Card.Content>
		<div class="overflow-x-auto rounded-md bg-muted p-4 font-mono text-sm whitespace-pre-wrap">
			{#if parsed.errorMessage}<div>{parsed.errorMessage}</div>{/if}
			{#each parsed.groups as group, i}
				{#if group.type === 'app'}
					<div>
						{#if group.frame.functionName}
							<div>{group.frame.functionName}</div>
						{/if}
						<div>{group.frame.location}</div>
					</div>
				{:else}
					<div
						class="-mb-4 cursor-pointer pt-2 text-muted-foreground select-none"
						role="button"
						tabindex="0"
						onclick={() => toggleGroup(i)}
						onkeydown={(e) => {
							if (e.key === 'Enter' || e.key === ' ') {
								e.preventDefault();
								toggleGroup(i);
							}
						}}
					>
						<span class="inline-flex items-center">
							{#if expandedGroups.has(i)}
								<ChevronDown class="mr-0.5 h-3.5 w-3.5" />
							{:else}
								<ChevronRight class="mr-0.5 h-3.5 w-3.5" />
							{/if}
							{group.frames.length} library {group.frames.length === 1 ? 'frame' : 'frames'} ({group.packageName})
						</span>
					</div>
					{#if expandedGroups.has(i)}
						{#each group.frames as frame}
							<div class="text-muted-foreground">
								{#if frame.functionName}
									<div>{frame.functionName}</div>
								{/if}
								<div>{frame.location}</div>
							</div>
						{/each}
					{/if}
				{/if}
			{/each}
		</div>
	</Card.Content>
</Card.Root>
