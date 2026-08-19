<script lang="ts">
	import * as Table from '$lib/components/ui/table';
	import { TracewayTableHeader } from '$lib/components/ui/traceway-table-header';
	import { TableEmptyState } from '$lib/components/ui/table-empty-state';
	import type {
		SessionActionEvent,
		SessionNetworkEvent,
		SessionNavigationEvent,
		SessionCustomEvent
	} from '$lib/types/exceptions';

	interface Props {
		actions: SessionActionEvent[];
		startedAt?: string;
		currentTimeMs?: number;
		onSeek?: (offsetMs: number) => void;
	}

	let { actions, startedAt, currentTimeMs, onSeek }: Props = $props();

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

	function typeColor(type: SessionActionEvent['type']): string {
		switch (type) {
			case 'network':
				return 'text-sky-600 dark:text-sky-400';
			case 'navigation':
				return 'text-violet-600 dark:text-violet-400';
			case 'custom':
				return 'text-amber-600 dark:text-amber-400';
			default:
				return 'text-foreground';
		}
	}

	function summary(a: SessionActionEvent): string {
		switch (a.type) {
			case 'network': {
				const n = a as SessionNetworkEvent;
				const status = n.statusCode ?? (n.error ? 'ERR' : '?');
				return `${n.method} ${n.url} → ${status} (${n.durationMs}ms)`;
			}
			case 'navigation': {
				const n = a as SessionNavigationEvent;
				const from = n.from ?? '?';
				const to = n.to ?? '?';
				return `${n.action}: ${from} → ${to}`;
			}
			case 'custom': {
				const c = a as SessionCustomEvent;
				const data = c.data ? ` ${JSON.stringify(c.data)}` : '';
				return `${c.category}/${c.name}${data}`;
			}
		}
	}

	const activeIndex = $derived.by(() => {
		if (currentTimeMs == null) return -1;
		let last = -1;
		for (let i = 0; i < actions.length; i++) {
			if (offsetMs(actions[i].timestamp) <= currentTimeMs) last = i;
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
		{#if actions.length > 0}
			<Table.Header class="sticky top-0 z-10 bg-background">
				<Table.Row>
					<TracewayTableHeader
						label="Time"
						tooltip="Offset from the start of the session recording"
						class="w-[110px]"
					/>
					<TracewayTableHeader
						label="Type"
						tooltip="Network / navigation / custom"
						class="w-[110px]"
					/>
					<TracewayTableHeader
						label="Details"
						tooltip="One-line summary; full payload visible on hover"
					/>
				</Table.Row>
			</Table.Header>
		{/if}
		<Table.Body>
			{#if actions.length === 0}
				<TableEmptyState colspan={3} message="No actions captured for this session." />
			{:else}
				{#each actions as entry, i (i)}
					<Table.Row
						onclick={() => onSeek?.(offsetMs(entry.timestamp))}
						class="cursor-pointer {i === activeIndex
							? 'border-l-2 border-l-primary bg-primary/10'
							: ''}"
					>
						<Table.Cell class="font-mono text-xs text-muted-foreground tabular-nums">
							{formatOffset(entry.timestamp)}
						</Table.Cell>
						<Table.Cell class="font-mono text-xs uppercase {typeColor(entry.type)}">
							{entry.type}
						</Table.Cell>
						<Table.Cell class="max-w-[600px] truncate font-mono text-xs" title={summary(entry)}>
							{summary(entry)}
						</Table.Cell>
					</Table.Row>
				{/each}
			{/if}
		</Table.Body>
	</Table.Root>
</div>
