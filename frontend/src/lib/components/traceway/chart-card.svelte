<script lang="ts">
	import type { Snippet } from 'svelte';
	import * as Card from '$lib/components/ui/card';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { cn } from '$lib/utils.js';

	interface Props {
		title: string;
		icon?: Snippet;
		controls?: Snippet;
		loading?: boolean;
		empty?: boolean;
		emptyMessage?: string;
		height?: number;
		class?: string;
		children: Snippet;
	}

	let {
		title,
		icon,
		controls,
		loading = false,
		empty = false,
		emptyMessage = 'No data in this period',
		height = 220,
		class: className,
		children
	}: Props = $props();
</script>

<Card.Root class={cn('gap-3 pt-2', className)}>
	<Card.Header
		class="flex flex-row items-center justify-between space-y-0 border-b pl-3 [.border-b]:pb-2"
	>
		<Card.Title class="flex items-center gap-2 text-base font-medium">
			{#if icon}{@render icon()}{/if}
			<span class="text-sm font-normal text-muted-foreground">{title}</span>
		</Card.Title>
		{#if controls}{@render controls()}{/if}
	</Card.Header>
	<Card.Content>
		{#if loading}
			<div class="flex items-center justify-center" style:height="{height}px">
				<LoadingCircle size="lg" />
			</div>
		{:else if empty}
			<div
				class="flex items-center justify-center text-sm text-muted-foreground"
				style:height="{height}px"
			>
				{emptyMessage}
			</div>
		{:else}
			{@render children()}
		{/if}
	</Card.Content>
</Card.Root>
