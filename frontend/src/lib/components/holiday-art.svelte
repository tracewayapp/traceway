<script lang="ts">
	import { cn } from '$lib/utils';
	import { holidaySeason } from '$lib/utils/holiday';

	let { name, class: className }: { name: string; class?: string } = $props();
</script>

{#if holidaySeason}
	<img
		src="/holiday/{name}.svg"
		alt=""
		aria-hidden="true"
		class={cn('pointer-events-none select-none', className)}
	/>
{/if}
