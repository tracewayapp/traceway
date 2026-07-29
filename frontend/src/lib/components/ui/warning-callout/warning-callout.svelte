<script lang="ts">
	import type { Snippet } from 'svelte';
	import { TriangleAlert, OctagonAlert } from 'lucide-svelte';
	import { cn } from '$lib/utils.js';

	let {
		title,
		variant = 'warning',
		children,
		action,
		class: className
	}: {
		title?: string;
		variant?: 'warning' | 'destructive';
		children: Snippet;
		action?: Snippet;
		class?: string;
	} = $props();

	const destructive = $derived(variant === 'destructive');
</script>

<div
	class={cn(
		'overflow-hidden rounded-md border bg-card',
		destructive ? 'border-destructive' : 'border-[#ffab01]',
		className
	)}
	role="alert"
>
	<div class="flex items-stretch">
		<div
			class="flex items-center justify-center px-3 text-white {destructive
				? 'bg-destructive'
				: 'bg-[#ffab01]'}"
		>
			{#if destructive}
				<OctagonAlert class="size-4" />
			{:else}
				<TriangleAlert class="size-4" />
			{/if}
		</div>
		<div class="flex min-w-0 flex-1 items-center justify-between gap-3 p-3">
			<div class="min-w-0 flex-1">
				{#if title}
					<div
						class="font-black leading-tight {destructive ? 'text-destructive' : 'text-[#ffab01]'}"
					>
						{title}
					</div>
				{/if}
				<div class="mt-0.5 text-sm text-muted-foreground">
					{@render children()}
				</div>
			</div>
			{#if action}
				{@render action()}
			{/if}
		</div>
	</div>
</div>
