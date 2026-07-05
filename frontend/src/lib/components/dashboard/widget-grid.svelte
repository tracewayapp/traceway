<script lang="ts">
	import * as Card from '$lib/components/ui/card';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import {
		Plus,
		EllipsisVertical,
		Pencil,
		Move,
		ArrowUp,
		ArrowDown,
		ArrowLeft,
		ArrowRight,
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
		onMoveWidget,
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
		onMoveWidget?: (widgetId: number, offset: number) => void;
		onAddWidget?: () => void;
		onToggleStar?: (widget: Widget) => void;
		onRangeSelect?: (from: Date, to: Date) => void;
	}>();

	let sharedHoverTime = $state<Date | null>(null);
	let cols = $state(3);

	$effect(() => {
		const mql = window.matchMedia('(min-width: 768px)');
		const handler = (e: MediaQueryListEvent | MediaQueryList) => {
			cols = e.matches ? 3 : 1;
		};
		handler(mql);
		mql.addEventListener('change', handler);
		return () => mql.removeEventListener('change', handler);
	});

	const sortedWidgets = $derived([...widgets].sort((a, b) => a.position - b.position));

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

<div class="grid grid-cols-1 gap-4 md:grid-cols-3">
	{#each sortedWidgets as widget, i (widget.id)}
		<div class={colSpanClass[widget.config?.colSpan ?? 1] ?? 'md:col-span-1'}>
			<Card.Root class="h-full gap-0 {minHeightClass[widget.config?.size ?? 'sm'] ?? 'min-h-[240px]'}">
				<Card.Header class="pr-2 pb-1">
					<div class="flex items-center justify-between">
						<Card.Title class="text-sm font-medium">{widget.title}{#if widget.config?.unit}<span class="text-xs font-normal text-muted-foreground"> ({widget.config.unit})</span>{/if}</Card.Title>
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
								<DropdownMenu.Sub>
									<DropdownMenu.SubTrigger>
										<Move class="mr-2 h-4 w-4" />
										Move
									</DropdownMenu.SubTrigger>
									<DropdownMenu.SubContent>
										<DropdownMenu.Item
											disabled={i < cols}
											onclick={() => onMoveWidget?.(widget.id, -cols)}
										>
											<ArrowUp class="mr-2 h-4 w-4" />
											Up
										</DropdownMenu.Item>
										<DropdownMenu.Item
											disabled={i > sortedWidgets.length - 1 - cols}
											onclick={() => onMoveWidget?.(widget.id, cols)}
										>
											<ArrowDown class="mr-2 h-4 w-4" />
											Down
										</DropdownMenu.Item>
										{#if cols > 1}
											<DropdownMenu.Item
												disabled={i % cols === 0}
												onclick={() => onMoveWidget?.(widget.id, -1)}
											>
												<ArrowLeft class="mr-2 h-4 w-4" />
												Left
											</DropdownMenu.Item>
											<DropdownMenu.Item
												disabled={i % cols === cols - 1 || i + 1 >= sortedWidgets.length}
												onclick={() => onMoveWidget?.(widget.id, 1)}
											>
												<ArrowRight class="mr-2 h-4 w-4" />
												Right
											</DropdownMenu.Item>
										{/if}
									</DropdownMenu.SubContent>
								</DropdownMenu.Sub>
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
				<Card.Content class="p-1">
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
