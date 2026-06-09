<script lang="ts">
	import * as Card from '$lib/components/ui/card';
	import { formatDateTime } from '$lib/utils/formatters';
	import { getTimezone } from '$lib/state/timezone.svelte';
	import Button from '../ui/button/button.svelte';
	import { Archive, ChevronRight, ChevronDown } from 'lucide-svelte';
	import { parseStackTrace, type StackFrame } from '$lib/utils/stack-trace-parser';

	interface Props {
		stackTrace: string;
		isMessage?: boolean;
		isJavaScript?: boolean;
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
		isJavaScript = false,
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
	const usePretty = $derived(isJavaScript && parsed.groups.length > 0);

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

	function formatFrame(frame: StackFrame) {
		const fn = (frame.functionName ?? '').replace(/\(\)\s*$/, '').trim() || '<anonymous>';
		const m = frame.location.match(/^(.*):(\d+):(\d+)$/);
		if (!m) {
			return { fn, dir: '', file: frame.location, lineCol: '', raw: frame.location };
		}
		const [, path, line, col] = m;
		const slash = path.lastIndexOf('/');
		return {
			fn,
			dir: slash >= 0 ? path.slice(0, slash + 1) : '',
			file: slash >= 0 ? path.slice(slash + 1) : path,
			lineCol: `:${line}:${col}`,
			raw: frame.location
		};
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
				<span class="tabular-nums"
					>First seen: {formatDateTime(firstSeen!, { timezone: tz })} · Last seen: {formatDateTime(
						lastSeen!,
						{ timezone: tz }
					)} · Total occurrences: {totalCount}</span
				>
			</Card.Description>
		{/if}
	</Card.Header>
	<Card.Content>
		{#if usePretty}
		<div class="overflow-hidden rounded-lg border">
			{#if parsed.errorMessage}
				<div class="border-b bg-muted/60 px-4 py-3">
					<p class="font-mono text-sm font-medium break-words whitespace-pre-wrap text-foreground">
						{parsed.errorMessage}
					</p>
				</div>
			{/if}

			<ol role="list" class="divide-y divide-border">
				{#each parsed.groups as group, i}
					{#if group.type === 'app'}
						{@const f = formatFrame(group.frame)}
						<li
							class="flex flex-wrap items-baseline gap-x-2.5 gap-y-0.5 border-l-2 px-4 py-2.5 {i ===
							0
								? 'border-l-primary'
								: 'border-l-primary/35'}"
						>
							<div class="min-w-0 font-mono text-sm font-medium break-all text-foreground">
								{f.fn}
							</div>
							<div class="min-w-0 font-mono text-xs break-all text-muted-foreground tabular-nums" title={f.raw}>
								{f.dir}<span class="text-foreground/85">{f.file}</span>{f.lineCol}
							</div>
						</li>
					{:else}
						<li class="border-l-2 border-l-transparent">
							<button
								type="button"
								class="flex w-full items-center gap-1.5 px-4 py-2 text-left text-xs text-muted-foreground hover:bg-muted/60"
								onclick={() => toggleGroup(i)}
							>
								{#if expandedGroups.has(i)}
									<ChevronDown class="size-3.5 shrink-0" />
								{:else}
									<ChevronRight class="size-3.5 shrink-0" />
								{/if}
								<span class="tabular-nums"
									>{group.frames.length} library {group.frames.length === 1 ? 'frame' : 'frames'}</span
								>
								<span class="rounded bg-muted px-1.5 py-0.5 font-mono text-foreground/70"
									>{group.packageName}</span
								>
							</button>
							{#if expandedGroups.has(i)}
								<ol role="list" class="divide-y divide-border/50 border-t border-border/50 bg-muted/20">
									{#each group.frames as frame}
										{@const f = formatFrame(frame)}
										<li class="flex flex-wrap items-baseline gap-x-2.5 gap-y-0.5 py-2 pr-4 pl-9">
											<div class="min-w-0 font-mono text-sm break-all text-muted-foreground">
												{f.fn}
											</div>
											<div
												class="min-w-0 font-mono text-xs break-all text-muted-foreground/70 tabular-nums"
												title={f.raw}
											>
												{f.dir}<span class="text-foreground/60">{f.file}</span>{f.lineCol}
											</div>
										</li>
									{/each}
								</ol>
							{/if}
						</li>
					{/if}
				{/each}
			</ol>
		</div>
		{:else}
		<div class="overflow-x-auto rounded-lg border bg-muted/40 p-4">
			<pre class="font-mono text-sm break-words whitespace-pre-wrap text-foreground">{stackTrace}</pre>
		</div>
		{/if}
	</Card.Content>
</Card.Root>
