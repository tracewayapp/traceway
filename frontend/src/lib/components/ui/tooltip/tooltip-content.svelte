<script lang="ts">
	import { Tooltip as TooltipPrimitive } from 'bits-ui';
	import { cn } from '$lib/utils.js';
	import TooltipPortal from './tooltip-portal.svelte';
	import type { ComponentProps } from 'svelte';
	import type { WithoutChildrenOrChild } from '$lib/utils.js';

	let {
		ref = $bindable(null),
		class: className,
		sideOffset = 0,
		side = 'top',
		children,
		arrowClasses,
		portalProps,
		...restProps
	}: TooltipPrimitive.ContentProps & {
		arrowClasses?: string;
		portalProps?: WithoutChildrenOrChild<ComponentProps<typeof TooltipPortal>>;
	} = $props();
</script>

<TooltipPortal {...portalProps}>
	<TooltipPrimitive.Content
		bind:ref
		data-slot="tooltip-content"
		{sideOffset}
		{side}
		class={cn(
			'z-50 max-w-[220px] origin-(--bits-tooltip-content-transform-origin) animate-in rounded-sm border border-transparent bg-foreground px-3 py-2 text-xs text-background shadow-[none] fade-in-0 zoom-in-95 data-[side=bottom]:slide-in-from-top-2 data-[side=left]:slide-in-from-end-2 data-[side=right]:slide-in-from-start-2 data-[side=top]:slide-in-from-bottom-2 data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=closed]:zoom-out-95 dark:border-border',
			className
		)}
		{...restProps}
	>
		{@render children?.()}
		<TooltipPrimitive.Arrow>
			{#snippet child({ props })}
				<div
					class={cn(
						'z-50 size-2.5 rotate-45 border-transparent bg-foreground dark:border-border',
						'data-[side=top]:border-r data-[side=top]:border-b',
						'data-[side=bottom]:border-t data-[side=bottom]:border-l',
						'data-[side=left]:border-r data-[side=left]:border-b',
						'data-[side=right]:border-r data-[side=right]:border-b',
						'data-[side=top]:translate-x-1/2 data-[side=top]:translate-y-[calc(-50%_+_3px)]',
						'data-[side=bottom]:-translate-x-1/2 data-[side=bottom]:-translate-y-[calc(50%_-_2px)] data-[side=bottom]:rotate-225',
						'data-[side=right]:translate-x-[calc(50%_+_2px)] data-[side=right]:translate-y-1/2',
						'data-[side=left]:-translate-y-[calc(50%_-_3px)]',
						arrowClasses
					)}
					{...props}
				></div>
			{/snippet}
		</TooltipPrimitive.Arrow>
	</TooltipPrimitive.Content>
</TooltipPortal>
