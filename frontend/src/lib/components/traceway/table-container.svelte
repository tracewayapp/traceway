<script lang="ts">
	import type { Snippet } from 'svelte';
	import { cn } from '$lib/utils.js';

	interface Props {
		class?: string;
		minWidth?: string;
		/** True while there are no real (wide) columns to show - e.g. a loading spinner
		 * or "no data" row. Drops the min-width so narrow viewports don't get a
		 * pointless horizontal scrollbar under a message that doesn't need one. */
		empty?: boolean;
		children: Snippet;
	}

	let { class: className, minWidth, empty = false, children }: Props = $props();
</script>

<!-- Wide tables scroll inside this container instead of squishing their columns
     or silently clipping them at the viewport edge. -->
<div class={cn('overflow-x-auto rounded-md border', className)}>
	<div style:min-width={empty ? undefined : minWidth}>
		{@render children()}
	</div>
</div>
