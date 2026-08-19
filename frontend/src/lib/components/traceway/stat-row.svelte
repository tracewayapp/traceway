<script lang="ts">
	import type { Snippet } from 'svelte';
	import * as Card from '$lib/components/ui/card';
	import { cn } from '$lib/utils.js';

	interface Props {
		columns?: number;
		class?: string;
		children: Snippet;
	}

	let { columns = 4, class: className, children }: Props = $props();
</script>

<Card.Root class={cn('py-0', className)}>
	<div
		class="grid grid-cols-2 divide-y divide-border sm:grid-cols-[repeat(var(--stat-cols),minmax(0,1fr))] sm:divide-x sm:divide-y-0"
		style="--stat-cols: {columns}"
	>
		{@render children()}
	</div>
</Card.Root>
