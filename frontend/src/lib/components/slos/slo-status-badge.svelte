<script lang="ts">
	import type { SloStatus } from '$lib/state/slos.svelte';

	interface Props {
		status: SloStatus;
		enabled?: boolean;
	}

	let { status, enabled = true }: Props = $props();

	const pillBase = 'inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-xs font-medium';
</script>

{#if !enabled}
	<span class="{pillBase} bg-muted text-muted-foreground">
		<span class="h-1.5 w-1.5 rounded-full bg-muted-foreground/60"></span>
		Paused
	</span>
{:else if status === 'ok'}
	<span class="{pillBase} bg-green-500/15 text-green-600 dark:text-green-400">
		<span class="h-1.5 w-1.5 rounded-full bg-green-500"></span>
		Meeting
	</span>
{:else if status === 'breached'}
	<span class="{pillBase} bg-red-500/15 text-red-600 dark:text-red-400">
		<span class="h-1.5 w-1.5 rounded-full bg-red-500"></span>
		Breached
	</span>
{:else}
	<span class="{pillBase} bg-muted text-muted-foreground">
		<span class="h-1.5 w-1.5 rounded-full bg-muted-foreground/60"></span>
		No data
	</span>
{/if}
