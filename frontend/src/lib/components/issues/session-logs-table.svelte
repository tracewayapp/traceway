<script lang="ts">
	import * as Table from '$lib/components/ui/table';
	import { TracewayTableHeader } from '$lib/components/ui/traceway-table-header';
	import { TableEmptyState } from '$lib/components/ui/table-empty-state';
	import type { SessionLogEvent, SessionLogLevel } from '$lib/types/exceptions';

	interface Props {
		logs: SessionLogEvent[];
		startedAt?: string;
		currentTimeMs?: number;
		onSeek?: (offsetMs: number) => void;
	}

	let { logs, startedAt, currentTimeMs, onSeek }: Props = $props();

	let scrollEl: HTMLDivElement | undefined = $state();

	function offsetMs(timestamp: string): number {
		if (!startedAt) return 0;
		const start = Date.parse(startedAt);
		const t = Date.parse(timestamp);
		if (!Number.isFinite(start) || !Number.isFinite(t)) return 0;
		return t - start;
	}

	function formatOffset(timestamp: string): string {
		if (!startedAt) return timestamp;
		const start = Date.parse(startedAt);
		const t = Date.parse(timestamp);
		if (!Number.isFinite(start) || !Number.isFinite(t)) return timestamp;
		const deltaMs = t - start;
		if (Math.abs(deltaMs) >= 1000) {
			const sign = deltaMs >= 0 ? '+' : '-';
			return `${sign}${(Math.abs(deltaMs) / 1000).toFixed(2)}s`;
		}
		const sign = deltaMs >= 0 ? '+' : '-';
		return `${sign}${Math.abs(deltaMs)}ms`;
	}

	function levelColor(level: SessionLogLevel): string {
		switch (level) {
			case 'error':
				return 'text-destructive';
			case 'warn':
				return 'text-yellow-600 dark:text-yellow-400';
			case 'debug':
				return 'text-muted-foreground';
			default:
				return 'text-foreground';
		}
	}

	const activeIndex = $derived.by(() => {
		if (currentTimeMs == null) return -1;
		let last = -1;
		for (let i = 0; i < logs.length; i++) {
			if (offsetMs(logs[i].timestamp) <= currentTimeMs) last = i;
			else break;
		}
		return last;
	});

	$effect(() => {
		const i = activeIndex;
		if (i < 0 || !scrollEl) return;
		const el = scrollEl.querySelectorAll<HTMLTableRowElement>('tbody tr')[i];
		if (!el) return;
		const top = el.offsetTop;
		const bottom = top + el.offsetHeight;
		if (top < scrollEl.scrollTop) {
			scrollEl.scrollTo({ top, behavior: 'smooth' });
		} else if (bottom > scrollEl.scrollTop + scrollEl.clientHeight) {
			scrollEl.scrollTo({ top: bottom - scrollEl.clientHeight, behavior: 'smooth' });
		}
	});
</script>

<div bind:this={scrollEl} class="max-h-[440px] overflow-y-auto rounded-md border">
	<Table.Root>
		{#if logs.length > 0}
			<Table.Header class="sticky top-0 z-10 bg-background">
				<Table.Row>
					<TracewayTableHeader
						label="Time"
						tooltip="Offset from the start of the session recording"
						class="w-[110px]"
					/>
					<TracewayTableHeader
						label="Level"
						tooltip="Console severity (debug / info / warn / error)"
						class="w-[80px]"
					/>
					<TracewayTableHeader label="Message" tooltip="The console line as captured" />
				</Table.Row>
			</Table.Header>
		{/if}
		<Table.Body>
			{#if logs.length === 0}
				<TableEmptyState colspan={3} message="No logs captured for this session." />
			{:else}
				{#each logs as entry, i}
					<Table.Row
						onclick={() => onSeek?.(offsetMs(entry.timestamp))}
						class="cursor-pointer hover:bg-muted/50 {i === activeIndex
							? 'border-l-2 border-l-primary bg-primary/10'
							: ''}"
					>
						<Table.Cell class="font-mono text-xs text-muted-foreground tabular-nums">
							{formatOffset(entry.timestamp)}
						</Table.Cell>
						<Table.Cell class="font-mono text-xs uppercase {levelColor(entry.level)}">
							{entry.level}
						</Table.Cell>
						<Table.Cell
							class="font-mono text-xs break-all whitespace-pre-wrap"
							title={entry.message}
						>
							{entry.message}
						</Table.Cell>
					</Table.Row>
				{/each}
			{/if}
		</Table.Body>
	</Table.Root>
</div>
