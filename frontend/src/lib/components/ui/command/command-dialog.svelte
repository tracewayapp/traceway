<script lang="ts">
	import { Dialog as DialogPrimitive, type WithoutChildrenOrChild } from 'bits-ui';
	import type { Snippet } from 'svelte';
	import Command from './command.svelte';
	import { cn } from '$lib/utils.js';

	let {
		open = $bindable(false),
		ref = $bindable(null),
		value = $bindable(''),
		title = 'Command Palette',
		description = 'Search for a command to run',
		class: className,
		portalProps,
		children,
		...restProps
	}: WithoutChildrenOrChild<DialogPrimitive.RootProps> & {
		portalProps?: DialogPrimitive.PortalProps;
		ref?: HTMLDivElement | null;
		value?: string;
		title?: string;
		description?: string;
		class?: string;
		children: Snippet;
	} = $props();
</script>

<DialogPrimitive.Root bind:open {...restProps}>
	<DialogPrimitive.Portal {...portalProps}>
		<DialogPrimitive.Overlay
			class="fixed inset-0 z-50 bg-black/50 data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:animate-in data-[state=open]:fade-in-0"
		/>
		<DialogPrimitive.Content
			class={cn(
				'fixed top-1/2 left-1/2 z-50 w-full max-w-lg -translate-x-1/2 -translate-y-1/2 overflow-hidden border bg-background p-0 shadow-lg duration-200 data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=closed]:zoom-out-95 data-[state=open]:animate-in data-[state=open]:fade-in-0 data-[state=open]:zoom-in-95 sm:rounded-md',
				className
			)}
		>
			<DialogPrimitive.Title class="sr-only">{title}</DialogPrimitive.Title>
			<DialogPrimitive.Description class="sr-only">{description}</DialogPrimitive.Description>
			<Command
				bind:ref
				bind:value
				class="[&_[data-command-group]]:px-2 [&_[data-command-group]:not([hidden])_~[data-command-group]]:pt-0 [&_[data-command-input-wrapper]_svg]:h-5 [&_[data-command-input-wrapper]_svg]:w-5 [&_[data-command-input]]:h-12 [&_[data-command-item]]:px-2 [&_[data-command-item]]:py-3 [&_[data-command-item]_svg]:h-5 [&_[data-command-item]_svg]:w-5"
			>
				{@render children?.()}
			</Command>
		</DialogPrimitive.Content>
	</DialogPrimitive.Portal>
</DialogPrimitive.Root>
