<script lang="ts">
	import * as Card from '$lib/components/ui/card';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import {
		Plus,
		EllipsisVertical,
		Pencil,
		Copy,
		GripVertical,
		Star,
		Trash2,
		ArrowUp,
		ArrowDown,
		ArrowLeftRight,
		ArrowUpDown,
		Check
	} from '@lucide/svelte';
	import WidgetRenderer from './widget-renderer.svelte';
	import type { DashboardWidgetConfig } from '$lib/types/dashboard';

	type Widget = {
		id: number | string;
		title: string;
		widgetType: string;
		config: DashboardWidgetConfig;
		position: number;
		isStarred?: boolean;
	};

	let {
		widgets = [],
		fromDateUTC,
		toDateUTC,
		timeDomain = null,
		onEditWidget,
		onDeleteWidget,
		onReorderWidgets,
		onDuplicateWidget,
		onResizeWidget,
		onAddWidget,
		onToggleStar,
		onRangeSelect,
		scopeTagFilters = {}
	} = $props<{
		widgets: Widget[];
		fromDateUTC: string;
		toDateUTC: string;
		timeDomain: [Date, Date] | null;
		onEditWidget?: (widget: Widget) => void;
		onDeleteWidget?: (widget: Widget) => void;
		onReorderWidgets?: (widgetIds: (number | string)[]) => void;
		onDuplicateWidget?: (widget: Widget) => void;
		onResizeWidget?: (widget: Widget, layout: { colSpan: number; size: string }) => void;
		onAddWidget?: () => void;
		onToggleStar?: (widget: Widget) => void;
		onRangeSelect?: (from: Date, to: Date) => void;
		scopeTagFilters?: Record<string, string>;
	}>();

	let sharedHoverTime = $state<Date | null>(null);

	const sortedWidgets = $derived([...widgets].sort((a, b) => a.position - b.position));

	function widgetAggregation(widget: Widget): string | null {
		if (widget.widgetType !== 'gauge' && widget.widgetType !== 'single_value') {
			return null;
		}
		const aggregations = new Set<string>(
			(widget.config?.sources ?? []).map((s: { aggregation?: string }) => s.aggregation || 'avg')
		);
		return aggregations.size === 1 ? [...aggregations][0] : null;
	}

	let dragIndex = $state<number | null>(null);
	let dropIndex = $state<number | null>(null);
	let cardEls: HTMLElement[] = [];

	function handleDragStart(e: DragEvent, i: number) {
		dragIndex = i;
		if (e.dataTransfer) {
			e.dataTransfer.effectAllowed = 'move';
			e.dataTransfer.setData('text/plain', String(sortedWidgets[i].id));
			const card = cardEls[i];
			if (card) {
				e.dataTransfer.setDragImage(card, 20, 20);
			}
		}
	}

	function handleDragOver(e: DragEvent, i: number) {
		if (dragIndex === null) return;
		e.preventDefault();
		if (e.dataTransfer) e.dataTransfer.dropEffect = 'move';
		dropIndex = i;
	}

	function handleDrop(e: DragEvent, targetIndex: number) {
		e.preventDefault();
		if (dragIndex !== null && dragIndex !== targetIndex) {
			const order = sortedWidgets.map((w: Widget) => w.id);
			const [moved] = order.splice(dragIndex, 1);
			order.splice(targetIndex, 0, moved);
			onReorderWidgets?.(order);
		}
		dragIndex = null;
		dropIndex = null;
	}

	function handleDragEnd() {
		dragIndex = null;
		dropIndex = null;
	}

	function moveWidget(i: number, delta: number) {
		const target = i + delta;
		if (target < 0 || target >= sortedWidgets.length) return;
		const order = sortedWidgets.map((w: Widget) => w.id);
		[order[i], order[target]] = [order[target], order[i]];
		onReorderWidgets?.(order);
	}

	const hasMenu = $derived(
		!!(onEditWidget || onDuplicateWidget || onDeleteWidget || onReorderWidgets || onResizeWidget)
	);

	const widthOptions: { value: number; label: string }[] = [
		{ value: 1, label: '1 column' },
		{ value: 2, label: '2 columns' },
		{ value: 3, label: '3 columns (full)' }
	];
	const heightOptions: { value: string; label: string }[] = [
		{ value: 'sm', label: 'Small' },
		{ value: 'md', label: 'Medium' },
		{ value: 'lg', label: 'Large' }
	];

	// Static maps - Tailwind can't extract dynamically-built class names
	const colSpanClass: Record<number, string> = {
		1: 'md:col-span-1',
		2: 'md:col-span-2',
		3: 'md:col-span-3'
	};
	const minHeightClass: Record<string, string> = {
		sm: 'min-h-[240px]',
		md: 'min-h-[340px]',
		lg: 'min-h-[460px]'
	};
</script>

<div class="grid grid-cols-1 gap-4 md:grid-cols-3" role="list">
	{#each sortedWidgets as widget, i (widget.id)}
		<div
			bind:this={cardEls[i]}
			class={colSpanClass[widget.config?.colSpan ?? 1] ?? 'md:col-span-1'}
			role="listitem"
			ondragover={(e) => handleDragOver(e, i)}
			ondrop={(e) => handleDrop(e, i)}
		>
			<Card.Root
				class="group h-full gap-0 {minHeightClass[widget.config?.size ?? 'sm'] ??
					'min-h-[240px]'} transition-opacity {dragIndex === i ? 'opacity-40' : ''} {dropIndex ===
					i &&
				dragIndex !== null &&
				dragIndex !== i
					? 'ring-2 ring-primary'
					: ''}"
			>
				<Card.Header class="pr-2 pb-1">
					<div class="flex items-center justify-between">
						<div class="flex min-w-0 items-center gap-1">
							{#if onReorderWidgets}
								<span
									class="-ml-1 inline-flex h-7 w-5 shrink-0 cursor-grab items-center justify-center rounded text-muted-foreground/60 opacity-0 transition-opacity group-hover:opacity-100 hover:text-foreground focus-visible:opacity-100 active:cursor-grabbing"
									title="Drag to reorder"
									role="button"
									tabindex={-1}
									aria-label="Drag to reorder widget"
									draggable="true"
									ondragstart={(e) => handleDragStart(e, i)}
									ondragend={handleDragEnd}
								>
									<GripVertical class="h-4 w-4" />
								</span>
							{/if}
							<Card.Title class="truncate text-sm font-medium"
								>{widget.title}{#if widget.config?.unit}<span
										class="text-xs font-normal text-muted-foreground"
									>
										({widget.config.unit})</span
									>{/if}{#if widgetAggregation(widget)}<span
										class="text-xs font-normal text-muted-foreground"
										>&nbsp;&middot; {widgetAggregation(widget)}</span
									>{/if}</Card.Title
							>
						</div>
						<div class="flex items-center">
							{#if onToggleStar}
								<button
									type="button"
									aria-label={widget.isStarred ? 'Unstar widget' : 'Star widget'}
									title={widget.isStarred ? 'Unstar' : 'Star'}
									class="inline-flex h-7 w-7 items-center justify-center rounded-md transition-colors hover:bg-muted {widget.isStarred
										? 'text-yellow-400 hover:text-yellow-500'
										: 'text-muted-foreground hover:text-foreground'}"
									onclick={() => onToggleStar?.(widget)}
								>
									<Star class="h-4 w-4 {widget.isStarred ? 'fill-current' : ''}" />
								</button>
							{/if}
							{#if hasMenu}
								<DropdownMenu.Root>
									<DropdownMenu.Trigger
										class="inline-flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
									>
										<EllipsisVertical class="h-4 w-4" />
									</DropdownMenu.Trigger>
									<DropdownMenu.Content align="end">
										{#if onEditWidget}
											<DropdownMenu.Item onclick={() => onEditWidget?.(widget)}>
												<Pencil class="mr-2 h-4 w-4" />
												Edit
											</DropdownMenu.Item>
										{/if}
										{#if onDuplicateWidget}
											<DropdownMenu.Item onclick={() => onDuplicateWidget?.(widget)}>
												<Copy class="mr-2 h-4 w-4" />
												Duplicate
											</DropdownMenu.Item>
										{/if}
										{#if onReorderWidgets}
											<DropdownMenu.Item disabled={i === 0} onclick={() => moveWidget(i, -1)}>
												<ArrowUp class="mr-2 h-4 w-4" />
												Move up
											</DropdownMenu.Item>
											<DropdownMenu.Item
												disabled={i === sortedWidgets.length - 1}
												onclick={() => moveWidget(i, 1)}
											>
												<ArrowDown class="mr-2 h-4 w-4" />
												Move down
											</DropdownMenu.Item>
										{/if}
										{#if onResizeWidget}
											<DropdownMenu.Sub>
												<DropdownMenu.SubTrigger>
													<ArrowLeftRight class="mr-2 h-4 w-4" />
													Width
												</DropdownMenu.SubTrigger>
												<DropdownMenu.SubContent>
													{#each widthOptions as option (option.value)}
														<DropdownMenu.Item
															onclick={() =>
																onResizeWidget?.(widget, {
																	colSpan: option.value,
																	size: widget.config?.size ?? 'sm'
																})}
														>
															<Check
																class="mr-2 h-4 w-4 {(widget.config?.colSpan ?? 1) === option.value
																	? ''
																	: 'opacity-0'}"
															/>
															{option.label}
														</DropdownMenu.Item>
													{/each}
												</DropdownMenu.SubContent>
											</DropdownMenu.Sub>
											<DropdownMenu.Sub>
												<DropdownMenu.SubTrigger>
													<ArrowUpDown class="mr-2 h-4 w-4" />
													Height
												</DropdownMenu.SubTrigger>
												<DropdownMenu.SubContent>
													{#each heightOptions as option (option.value)}
														<DropdownMenu.Item
															onclick={() =>
																onResizeWidget?.(widget, {
																	colSpan: widget.config?.colSpan ?? 1,
																	size: option.value
																})}
														>
															<Check
																class="mr-2 h-4 w-4 {(widget.config?.size ?? 'sm') === option.value
																	? ''
																	: 'opacity-0'}"
															/>
															{option.label}
														</DropdownMenu.Item>
													{/each}
												</DropdownMenu.SubContent>
											</DropdownMenu.Sub>
										{/if}
										{#if onDeleteWidget}
											<DropdownMenu.Separator />
											<DropdownMenu.Item
												class="text-destructive"
												onclick={() => onDeleteWidget?.(widget)}
											>
												<Trash2 class="mr-2 h-4 w-4" />
												Delete
											</DropdownMenu.Item>
										{/if}
									</DropdownMenu.Content>
								</DropdownMenu.Root>
							{/if}
						</div>
					</div>
				</Card.Header>
				<Card.Content class="min-h-0 flex-1 p-1">
					<WidgetRenderer
						{widget}
						{fromDateUTC}
						{toDateUTC}
						{timeDomain}
						{onRangeSelect}
						{sharedHoverTime}
						{scopeTagFilters}
						onHoverTimeChange={(time) => (sharedHoverTime = time)}
						isSourceChart={false}
					/>
				</Card.Content>
			</Card.Root>
		</div>
	{/each}
	{#if onAddWidget}
		<button
			class="flex min-h-[240px] cursor-pointer items-center justify-center rounded-md border border-dashed border-muted-foreground/25 text-muted-foreground transition-colors hover:border-primary hover:text-primary"
			onclick={() => onAddWidget?.()}
		>
			<div class="flex flex-col items-center gap-2">
				<Plus class="h-8 w-8" />
				<span class="text-sm font-medium">Add Metric Widget</span>
			</div>
		</button>
	{/if}
</div>
