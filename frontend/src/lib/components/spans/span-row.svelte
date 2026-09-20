<script lang="ts">
	import type { Span, SpanAttributeLoader } from '$lib/types/spans';
	import { cn } from '$lib/utils';
	import { formatDuration, preciseTimeMs } from '$lib/utils/formatters';
	import * as Popover from '$lib/components/ui/popover';
	import CopyButton from '$lib/components/traceway/copy-button.svelte';
	import ChevronRight from '@lucide/svelte/icons/chevron-right';
	import ChevronDown from '@lucide/svelte/icons/chevron-down';
	import CircleAlert from '@lucide/svelte/icons/circle-alert';
	import Unlink from '@lucide/svelte/icons/unlink';
	import { spanDisplayName } from '$lib/utils/span-display';
	import { resolveHref } from '$lib/utils/links';
	import { isErrorSpan, spanKindLabel, spanStatusLabel } from '$lib/utils/span-fields';

	type Props = {
		row: number;
		span: Span;
		traceStart: number;
		traceDuration: number;
		isOdd: boolean;
		depth: number;
		hasChildren: boolean;
		missingParent?: boolean;
		missingParentLabel?: string;
		descendants?: number;
		errorBelow?: boolean;
		loadAttributes?: SpanAttributeLoader;
		projectName?: string;
		showService?: boolean;
		isSelected?: boolean;
		traceHref?: string;
		isCollapsed: boolean;
		onToggle: () => void;
		nameColumnWidth: number;
		updateNameWidth: (width: number) => void;

		spanCellHandleMouseEnter: (x: number) => void;
		spanCellHandleMouseMove: (x: number) => void;
		spanCellHandleMouseLeave: () => void;
	};

	let {
		row,
		span,
		traceStart,
		traceDuration,
		isOdd,
		depth,
		hasChildren,
		missingParent = false,
		missingParentLabel = 'Parent span is unavailable',
		descendants = 0,
		errorBelow = false,
		loadAttributes,
		projectName,
		showService = false,
		isSelected = false,
		traceHref,
		isCollapsed,
		onToggle,
		nameColumnWidth,
		updateNameWidth,
		spanCellHandleMouseEnter,
		spanCellHandleMouseMove,
		spanCellHandleMouseLeave
	}: Props = $props();

	const spanStartMs = $derived(preciseTimeMs(span.startTime) - traceStart);
	const spanDurationMs = $derived(span.duration / 1_000_000);
	const traceDurationMs = $derived(traceDuration / 1_000_000);

	// Calculate position and width as percentages
	const leftPercent = $derived(Math.max(0, (spanStartMs / traceDurationMs) * 100));
	const widthPercent = $derived(
		Math.min(100 - leftPercent, (spanDurationMs / traceDurationMs) * 100)
	);

	const spanColors = [
		{ bg: 'bg-blue-400', ring: 'ring-blue-500' },
		{ bg: 'bg-green-400', ring: 'ring-green-500' },
		{ bg: 'bg-purple-400', ring: 'ring-purple-500' },
		{ bg: 'bg-orange-400', ring: 'ring-orange-500' },
		{ bg: 'bg-red-400', ring: 'ring-red-500' },
		{ bg: 'bg-amber-400', ring: 'ring-amber-500' },
		{ bg: 'bg-cyan-400', ring: 'ring-cyan-500' },
		{ bg: 'bg-pink-400', ring: 'ring-pink-500' },
		{ bg: 'bg-indigo-400', ring: 'ring-indigo-500' },
		{ bg: 'bg-teal-400', ring: 'ring-teal-500' },
		{ bg: 'bg-lime-400', ring: 'ring-lime-500' },
		{ bg: 'bg-rose-400', ring: 'ring-rose-500' },
		{ bg: 'bg-sky-400', ring: 'ring-sky-500' },
		{ bg: 'bg-slate-400', ring: 'ring-slate-500' }
	];

	const spanColor = $derived(spanColors[row % spanColors.length]);

	// Tooltip state (this is the tooltip on top of the line)
	let isHovered = $state(false);
	let barElement: HTMLDivElement;

	function handleMouseEnter() {
		isHovered = true;
	}

	function handleMouseLeave() {
		isHovered = false;
	}

	let containerElement: HTMLDivElement;
	function containerSpanCellHandleMouseEnter(e: MouseEvent) {
		const rect = containerElement.getBoundingClientRect();
		const x = e.clientX - rect.left;
		spanCellHandleMouseEnter(x);
	}
	function containerSpanCellHandleMouseMove(e: MouseEvent) {
		const rect = containerElement.getBoundingClientRect();
		const x = e.clientX - rect.left;
		spanCellHandleMouseMove(x);
	}

	let nameElement: HTMLSpanElement;
	let pillElement = $state<HTMLButtonElement>();

	const isError = $derived(isErrorSpan(span));
	const fieldEntries = $derived(
		[
			['Project', projectName],
			['Service', span.serviceName],
			['Kind', spanKindLabel(span.spanKind)],
			['Status', spanStatusLabel(span.statusCode)],
			['Scope', span.scopeName]
		].filter((entry): entry is [string, string] => !!entry[1])
	);

	// Spans of a whole trace arrive without attributes. They are fetched once, when the popover first opens.
	let lazyAttributes = $state<Record<string, string> | null>(null);
	let lazyState = $state<'idle' | 'loading' | 'failed'>('idle');
	let lazyOmitted = $state(false);

	async function ensureAttributes(open: boolean) {
		if (!open || !loadAttributes || span.attributes || lazyAttributes || lazyState === 'loading') {
			return;
		}
		lazyState = 'loading';
		try {
			const loaded = await loadAttributes(span);
			lazyAttributes = loaded.attributes ?? {};
			lazyOmitted = !!loaded.attributesOmitted;
			lazyState = 'idle';
		} catch (e) {
			console.error(e);
			lazyState = 'failed';
		}
	}

	const attributeEntries = $derived(
		Object.entries(span.attributes ?? lazyAttributes ?? {}).sort((a, b) => a[0].localeCompare(b[0]))
	);

	const foldedCount = $derived(isCollapsed && hasChildren ? descendants : 0);

	$effect(() => {
		if (nameElement) {
			// Measure the natural width needed, including indentation and chevron
			const pillWidth = foldedCount > 0 && pillElement ? pillElement.offsetWidth + 12 : 0;
			const naturalWidth = nameElement.scrollWidth + depth * 16 + 44 + pillWidth;
			updateNameWidth?.(naturalWidth);
		}
	});
</script>

<div
	class={cn(
		'flex items-center border-b border-border last:border-b-0',
		isOdd ? 'bg-muted/40' : '',
		isSelected && 'bg-primary/10'
	)}
	aria-current={isSelected ? 'true' : undefined}
>
	<!-- Span name -->
	<div
		class="flex flex-shrink-0 items-center border-r border-border py-1.5 pr-3"
		style="min-width: {nameColumnWidth}px; max-width: {nameColumnWidth}px; padding-left: {12 +
			depth * 16}px"
	>
		{#if hasChildren}
			<button
				onclick={onToggle}
				class="mr-1 shrink-0 rounded p-0.5 text-muted-foreground hover:bg-muted hover:text-foreground"
				aria-label={isCollapsed ? 'Expand children' : 'Collapse children'}
				aria-expanded={!isCollapsed}
			>
				{#if isCollapsed}
					<ChevronRight class="h-3 w-3" />
				{:else}
					<ChevronDown class="h-3 w-3" />
				{/if}
			</button>
		{:else}
			<span class="mr-1 w-4 shrink-0"></span>
		{/if}
		<Popover.Root onOpenChange={ensureAttributes}>
			<Popover.Trigger class="min-w-0 flex-1 cursor-pointer text-left">
				<span bind:this={nameElement} class="block truncate font-mono text-xs">
					{#if isError}<CircleAlert
							class="mr-1 inline h-3 w-3 align-[-2px] text-destructive"
							aria-label="Error"
						/>{/if}{#if missingParent}<Unlink
							class="mr-1 inline h-3 w-3 align-[-2px] text-muted-foreground"
							aria-label={missingParentLabel}
						/>{/if}{#if showService && span.serviceName}<span class="mr-1.5 text-muted-foreground"
							>{span.serviceName}</span
						>{/if}{spanDisplayName(span)}
				</span>
			</Popover.Trigger>
			<Popover.Content class="w-auto max-w-sm" align="start">
				<div class="flex max-h-[60vh] flex-col gap-2 overflow-y-auto">
					{#if span.attributesOmitted || lazyOmitted || lazyState === 'failed'}<p
							class="text-xs text-muted-foreground"
						>
							Attributes could not be loaded for this span.
						</p>{/if}
					{#if missingParent}<p class="text-xs text-muted-foreground">{missingParentLabel}.</p>{/if}
					<div class="text-xs text-muted-foreground">
						Trace
						{#if traceHref}<a
								class="underline underline-offset-2"
								{...{ href: resolveHref(traceHref) }}><code>{span.traceId}</code></a
							>{:else}<code>{span.traceId}</code>{/if}
					</div>
					<div class="text-xs text-muted-foreground">Span <code>{span.spanId}</code></div>
					{#if fieldEntries.length > 0}
						<dl class="grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 text-xs">
							{#each fieldEntries as [label, value] (label)}
								<dt class="text-muted-foreground">{label}</dt>
								<dd
									class={cn(
										'font-mono break-all select-text',
										label === 'Status' && isError && 'text-destructive'
									)}
								>
									{value}
								</dd>
							{/each}
						</dl>
					{/if}
					<div class="flex items-start gap-2">
						<span class="font-mono text-xs break-all select-text">{span.name}</span>
						<CopyButton bare text={span.name} iconClass="h-3.5 w-3.5" class="shrink-0" />
					</div>
					{#if lazyState === 'loading'}
						<p class="border-t border-border pt-2 text-xs text-muted-foreground">
							Loading attributes...
						</p>
					{/if}
					{#if attributeEntries.length > 0}
						<div class="flex flex-col gap-1.5 border-t border-border pt-2">
							{#each attributeEntries as [key, value], __index (__index)}
								<div class="flex items-start gap-2">
									<div class="min-w-0 flex-1">
										<div class="font-mono text-xs break-all text-muted-foreground">{key}</div>
										<div class="font-mono text-xs break-all select-text">{value}</div>
									</div>
									<CopyButton bare text={value} iconClass="h-3.5 w-3.5" class="shrink-0" />
								</div>
							{/each}
						</div>
					{/if}
				</div>
			</Popover.Content>
		</Popover.Root>
		{#if foldedCount > 0}
			<button
				bind:this={pillElement}
				type="button"
				onclick={onToggle}
				class="ml-2 inline-flex shrink-0 cursor-pointer items-center gap-1 rounded-full bg-muted px-1.5 py-px font-mono text-[10px] text-muted-foreground hover:bg-muted-foreground/20 hover:text-foreground"
				aria-label={`Show ${foldedCount} hidden ${foldedCount === 1 ? 'span' : 'spans'}`}
			>
				{#if errorBelow}<CircleAlert
						class="h-2.5 w-2.5 text-destructive"
						aria-label="Error in the hidden spans"
					/>{/if}+{foldedCount.toLocaleString()}
			</button>
		{/if}
	</div>

	<!-- Timeline bar -->
	<div
		class="relative flex min-w-[200px] flex-1 items-center self-stretch"
		bind:this={containerElement}
		role="presentation"
		onmouseenter={containerSpanCellHandleMouseEnter}
		onmousemove={containerSpanCellHandleMouseMove}
		onmouseleave={spanCellHandleMouseLeave}
	>
		<div class="relative h-4 w-full">
			<div
				bind:this={barElement}
				class={cn(
					'absolute h-full rounded-[2px] transition-all',
					spanColor.bg,
					isHovered && `ring-2 ${spanColor.ring}`
				)}
				style="left: {leftPercent}%; width: {Math.max(widthPercent, 0.3)}%; min-width: 2px"
				onmouseenter={handleMouseEnter}
				onmouseleave={handleMouseLeave}
				role="presentation"
			></div>
		</div>
	</div>

	<!-- Duration -->
	<div
		class="w-[100px] flex-shrink-0 border-l border-border px-3 py-1.5 text-right font-mono text-xs text-muted-foreground"
	>
		{formatDuration(span.duration)}
	</div>
</div>
