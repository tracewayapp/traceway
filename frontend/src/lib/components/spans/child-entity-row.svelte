<script lang="ts">
	import type { ChildEntity } from '$lib/types/spans';
	import { cn } from '$lib/utils';
	import { formatDuration, preciseTimeMs } from '$lib/utils/formatters';
	import { addStickyParamsToHref } from '$lib/utils/navigation';
	import Globe from 'lucide-svelte/icons/globe';
	import Briefcase from 'lucide-svelte/icons/briefcase';
	import Sparkles from 'lucide-svelte/icons/sparkles';

	type Props = {
		row: number;
		entity: ChildEntity;
		traceStart: number;
		traceDuration: number;
		isOdd: boolean;
		nameColumnWidth: number;
	};

	let { row, entity, traceStart, traceDuration, isOdd, nameColumnWidth }: Props = $props();

	const entityStartMs = $derived(preciseTimeMs(entity.recordedAt) - traceStart);
	const entityDurationMs = $derived(entity.duration / 1_000_000);
	const traceDurationMs = $derived(traceDuration / 1_000_000);

	const leftPercent = $derived(Math.max(0, (entityStartMs / traceDurationMs) * 100));
	const widthPercent = $derived(
		Math.min(100 - leftPercent, (entityDurationMs / traceDurationMs) * 100)
	);

	const href = $derived(
		entity.kind === 'endpoint'
			? addStickyParamsToHref(
					`/endpoints/${encodeURIComponent(entity.name)}/${entity.id}`,
					'preset',
					'from',
					'to'
				)
			: entity.kind === 'task'
				? addStickyParamsToHref(
						`/tasks/${encodeURIComponent(entity.name)}/${entity.id}`,
						'preset',
						'from',
						'to'
					)
				: addStickyParamsToHref(
						`/ai-traces/${encodeURIComponent(entity.name)}/${entity.id}`,
						'preset',
						'from',
						'to'
					)
	);

	const kindLabel = $derived(
		entity.kind === 'ai_trace'
			? 'AI trace'
			: entity.kind.charAt(0).toUpperCase() + entity.kind.slice(1)
	);

	// Distinct color per kind so child entities are visually obvious next to
	// regular spans — and so they're consistent across rows of the same kind.
	const kindColor = $derived(
		entity.kind === 'endpoint'
			? { bg: 'bg-blue-500', ring: 'ring-blue-600' }
			: entity.kind === 'task'
				? { bg: 'bg-amber-500', ring: 'ring-amber-600' }
				: { bg: 'bg-fuchsia-500', ring: 'ring-fuchsia-600' }
	);

	let isHovered = $state(false);
</script>

<a
	{href}
	class={cn(
		'flex items-center border-b border-border last:border-b-0 hover:bg-muted/60',
		isOdd ? 'bg-muted/40' : ''
	)}
	title={`Open ${kindLabel}: ${entity.name}`}
>
	<!-- Entity name + icon -->
	<div
		class="flex flex-shrink-0 items-center gap-1.5 border-r border-border px-3 py-1.5 font-mono text-xs"
		style="min-width: {nameColumnWidth}px; max-width: {nameColumnWidth}px"
	>
		{#if entity.kind === 'endpoint'}
			<Globe class="h-3.5 w-3.5 shrink-0 text-blue-500" />
		{:else if entity.kind === 'task'}
			<Briefcase class="h-3.5 w-3.5 shrink-0 text-amber-500" />
		{:else}
			<Sparkles class="h-3.5 w-3.5 shrink-0 text-fuchsia-500" />
		{/if}
		<span class="min-w-0 flex-1 truncate text-left">{entity.name}</span>
		<span class="shrink-0 rounded bg-muted px-1 py-0.5 text-[10px] uppercase text-muted-foreground">
			{kindLabel}
		</span>
	</div>

	<!-- Timeline bar -->
	<div class="relative flex min-w-[200px] flex-1 items-center self-stretch">
		<div class="relative h-4 w-full">
			<div
				class={cn(
					'absolute h-full rounded-[2px] transition-all',
					kindColor.bg,
					isHovered && `ring-2 ${kindColor.ring}`
				)}
				style="left: {leftPercent}%; width: {Math.max(widthPercent, 0.3)}%; min-width: 2px"
				role="presentation"
				onmouseenter={() => (isHovered = true)}
				onmouseleave={() => (isHovered = false)}
			></div>
		</div>
	</div>

	<!-- Duration -->
	<div
		class="w-[100px] flex-shrink-0 border-l border-border px-3 py-1.5 text-right font-mono text-xs text-muted-foreground"
	>
		{formatDuration(entity.duration)}
	</div>
</a>
