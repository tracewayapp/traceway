<script lang="ts">
	import * as Card from '$lib/components/ui/card';
	import * as Tabs from '$lib/components/ui/tabs';
	import { formatDateTime } from '$lib/utils/formatters';
	import { getTimezone } from '$lib/state/timezone.svelte';
	import Button from '../ui/button/button.svelte';
	import { Archive, ChevronRight, ChevronDown } from 'lucide-svelte';
	import {
		parseStackTrace,
		looksLikeJava,
		looksLikeGo,
		type StackFrame
	} from '$lib/utils/stack-trace-parser';

	interface Props {
		stackTrace: string;
		isMessage?: boolean;
		isJavaScript?: boolean;
		isFlutter?: boolean;
		isIOS?: boolean;
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
		isFlutter = false,
		isIOS = false,
		firstSeen,
		lastSeen,
		totalCount,
		timezone,
		archiving = $bindable(),
		showArchiveDialog = $bindable()
	}: Props = $props();

	const tz = $derived(timezone ?? getTimezone());
	const showStats = $derived(firstSeen && lastSeen && totalCount !== undefined);
	const isJava = $derived(looksLikeJava(stackTrace));
	const isGo = $derived(looksLikeGo(stackTrace));
	const parsed = $derived(parseStackTrace(stackTrace, { ios: isIOS, java: isJava, go: isGo }));
	const usePretty = $derived(
		(isJavaScript || isFlutter || isIOS || isJava || isGo) && parsed.groups.length > 0
	);
	const groupNoun = $derived(isIOS || isJava ? 'system' : 'library');

	let viewMode = $state<'formatted' | 'raw'>('formatted');
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
		const fn = (frame.functionName ?? '').replace(/\(\)\s*$/, '').trim();
		const withCol = frame.location.match(/^(.*):(\d+):(\d+)$/);
		const m = withCol ?? frame.location.match(/^(.*):(\d+)$/);
		if (!m) {
			return { fn, dir: '', file: frame.location, lineCol: '', raw: frame.location };
		}
		const path = m[1];
		const lineCol = withCol ? `${m[2]}:${m[3]}` : m[2];
		const slash = path.lastIndexOf('/');
		return {
			fn,
			dir: slash >= 0 ? path.slice(0, slash + 1) : '',
			file: slash >= 0 ? path.slice(slash + 1) : path,
			lineCol,
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
			<div class="overflow-hidden rounded-md border">
				<div class="flex items-center justify-between gap-3 border-b bg-muted/50 px-4 py-3">
					{#if viewMode === 'formatted' && parsed.errorMessage}
						{@const messageColon = parsed.errorMessage.indexOf(':')}
						<p
							class="min-w-0 flex-1 font-mono text-[15px]/6 break-words whitespace-pre-wrap text-foreground"
						>
							{#if messageColon > 0}<span class="font-semibold text-rose-600 dark:text-rose-400"
									>{parsed.errorMessage.slice(0, messageColon)}</span
								>{parsed.errorMessage.slice(messageColon)}{:else}{parsed.errorMessage}{/if}
						</p>
					{:else}
						<div class="flex-1"></div>
					{/if}
					<div class="shrink-0">
						<Tabs.Root bind:value={viewMode}>
							<Tabs.List>
								<Tabs.Trigger value="formatted">Formatted</Tabs.Trigger>
								<Tabs.Trigger value="raw">Raw</Tabs.Trigger>
							</Tabs.List>
						</Tabs.Root>
					</div>
				</div>
				{#if viewMode === 'formatted'}
					<ol role="list" class="divide-y divide-border/60">
						{#each parsed.groups as group, i}
							{#if group.type === 'app'}
								{@const f = formatFrame(group.frame)}
								<li
									class="flex flex-wrap items-baseline gap-x-1.5 px-4 py-2.5 font-mono text-sm tabular-nums"
									title={f.raw}
								>
									<span class="min-w-0 break-all text-muted-foreground"
										>{f.dir}<span class="font-medium text-blue-600 dark:text-blue-400"
											>{f.file}</span
										></span
									>
									{#if f.fn}
										<span class="text-muted-foreground/70">in</span>
										<span class="min-w-0 font-medium break-all text-foreground">{f.fn}</span>
									{/if}
									{#if f.lineCol}
										<span class="text-muted-foreground/70">at line</span>
										<span class="font-medium text-amber-600 dark:text-amber-400">{f.lineCol}</span>
									{/if}
								</li>
							{:else}
								<li>
									<button
										type="button"
										class="flex w-full items-center gap-1.5 bg-muted/25 px-4 py-2 text-left text-xs text-muted-foreground hover:bg-muted/70"
										onclick={() => toggleGroup(i)}
									>
										{#if expandedGroups.has(i)}
											<ChevronDown class="size-3.5 shrink-0" />
										{:else}
											<ChevronRight class="size-3.5 shrink-0" />
										{/if}
										<span class="tabular-nums"
											>{group.frames.length}
											{groupNoun}
											{group.frames.length === 1 ? 'frame' : 'frames'}</span
										>
										<span
											class="rounded-md border bg-background px-1.5 py-0.5 font-mono text-foreground/70"
											>{group.packageName}</span
										>
									</button>
									{#if expandedGroups.has(i)}
										<ol
											role="list"
											class="divide-y divide-border/40 border-t border-border/40 bg-muted/30"
										>
											{#each group.frames as frame}
												{@const f = formatFrame(frame)}
												<li
													class="flex flex-wrap items-baseline gap-x-1.5 py-2 pr-4 pl-9 font-mono text-sm text-muted-foreground tabular-nums"
													title={f.raw}
												>
													<span class="min-w-0 break-all"
														>{f.dir}<span class="text-foreground/70">{f.file}</span></span
													>
													{#if f.fn}
														<span class="text-muted-foreground/60">in</span>
														<span class="min-w-0 break-all text-foreground/70">{f.fn}</span>
													{/if}
													{#if f.lineCol}
														<span class="text-muted-foreground/60">at line</span>
														<span>{f.lineCol}</span>
													{/if}
												</li>
											{/each}
										</ol>
									{/if}
								</li>
							{/if}
						{/each}
					</ol>
				{:else}
					<div class="overflow-x-auto bg-muted/40">
						<pre
							class="w-fit min-w-full p-4 font-mono text-sm whitespace-pre text-foreground">{stackTrace}</pre>
					</div>
				{/if}
			</div>
		{:else}
			<div class="overflow-x-auto rounded-md border bg-muted/40">
				<pre
					class="w-fit min-w-full p-4 font-mono text-sm whitespace-pre text-foreground">{stackTrace}</pre>
			</div>
		{/if}
	</Card.Content>
</Card.Root>
