<script lang="ts">
	import type { Snippet } from 'svelte';
	import { cn } from '$lib/utils.js';

	interface Props {
		class?: string;
		minWidth?: string;
		children: Snippet;
	}

	let { class: className, minWidth, children }: Props = $props();
</script>

<!-- Wide tables scroll inside this container instead of squishing their columns
     or silently clipping them at the viewport edge. -->
<div class={cn('overflow-x-auto rounded-md border', className)}>
	<div style:min-width={minWidth}>
		{@render children()}
	</div>
</div>
