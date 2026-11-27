<script lang="ts">
	import { onMount, type Component } from 'svelte';

	let Slot = $state<Component | null>(null);

	onMount(() => {
		import('$billing/signup-slot.svelte')
			.then((module) => {
				Slot = module.default;
			})
			.catch(() => {
				// Billing extension not available - expected for open source builds
			});
	});
</script>

{#if Slot}
	<Slot />
{/if}
