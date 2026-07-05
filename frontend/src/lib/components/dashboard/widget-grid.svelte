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
		Trash2
	} from 'lucide-svelte';
	import WidgetRenderer from './widget-renderer.svelte';

	type Widget = {
		id: number;
		title: string;
		widgetType: string;
		config: any;
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
		onAddWidget,
		onToggleStar,
		onRangeSelect
	} = $props<{
		widgets: Widget[];
		fromDateUTC: string;
		toDateUTC: string;
		timeDomain: [Date, Date] | null;
		onEditWidget?: (widget: Widget) => void;
		onDeleteWidget?: (widget: Widget) => void;
		onReorderWidgets?: (widgetIds: number[]) => void;
		onDuplicateWidget?: (widget: Widget) => void;
		onAddWidget?: () => void;
		onToggleStar?: (widget: Widget) => void;
		onRangeSelect?: (from: Date, to: Date) => void;
	}>();

	let sharedHoverTime = $state<Date | null>(null);

	const sortedWidgets = $derived([...widgets].sort((a, b) => a.position - b.position));

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

	// Static maps — Tailwind can't extract dynamically-built class names
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
				class="h-full gap-0 {minHeightClass[widget.config?.size ?? 'sm'] ?? 'min-h-[240px]'} transition-opacity {dragIndex === i ? 'opacity-40' : ''} {dropIndex === i && dragIndex !== null && dragIndex !== i ? 'ring-2 ring-primary' : ''}"
			>
				<Card.Header class="pr-2 pb-1">
					<div class="flex items-center justify-between">
						<div class="flex min-w-0 items-center gap-1">
							<span
								class="-ml-1 inline-flex h-7 w-5 shrink-0 cursor-grab items-center justify-center rounded text-muted-foreground/60 hover:text-foreground active:cursor-grabbing"
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
							<Card.Title class="truncate text-sm font-medium">{widget.title}{#if widget.config?.unit}<span class="text-xs font-normal text-muted-foreground"> ({widget.config.unit})</span>{/if}</Card.Title>
						</div>
						<div class="flex items-center">
							<button
								type="button"
								aria-label={widget.isStarred ? 'Unstar widget' : 'Star widget'}
								title={widget.isStarred ? 'Unstar' : 'Star'}
								class="inline-flex h-7 w-7 items-center justify-center rounded-md transition-colors hover:bg-muted {widget.isStarred ? 'text-yellow-500 hover:text-yellow-600' : 'text-muted-foreground hover:text-foreground'}"
								onclick={() => onToggleStar?.(widget)}
							>
								<Star class="h-4 w-4 {widget.isStarred ? 'fill-current' : ''}" />
							</button>
							<DropdownMenu.Root>
							<DropdownMenu.Trigger
								class="inline-flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
							>
								<EllipsisVertical class="h-4 w-4" />
							</DropdownMenu.Trigger>
							<DropdownMenu.Content align="end">
								<DropdownMenu.Item onclick={() => onEditWidget?.(widget)}>
									<Pencil class="mr-2 h-4 w-4" />
									Edit
								</DropdownMenu.Item>
								<DropdownMenu.Item onclick={() => onDuplicateWidget?.(widget)}>
									<Copy class="mr-2 h-4 w-4" />
									Duplicate
								</DropdownMenu.Item>
								<DropdownMenu.Separator />
								<DropdownMenu.Item
									class="text-destructive"
									onclick={() => onDeleteWidget?.(widget)}
								>
									<Trash2 class="mr-2 h-4 w-4" />
									Delete
								</DropdownMenu.Item>
							</DropdownMenu.Content>
						</DropdownMenu.Root>
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
						onHoverTimeChange={(time) => (sharedHoverTime = time)}
						isSourceChart={false}
					/>
				</Card.Content>
			</Card.Root>
		</div>
	{/each}
	<button
		class="flex min-h-[240px] cursor-pointer items-center justify-center rounded-lg border border-dashed border-muted-foreground/25 text-muted-foreground transition-colors hover:border-primary hover:text-primary"
		onclick={() => onAddWidget?.()}
	>
		<div class="flex flex-col items-center gap-2">
			<Plus class="h-8 w-8" />
			<span class="text-sm font-medium">Add Metric Widget</span>
		</div>
	</button>
</div>
