<script lang="ts">
	import { Tooltip as TooltipPrimitive } from 'bits-ui';
	import { getContext } from 'svelte';

	let { ref = $bindable(null), ...restProps }: TooltipPrimitive.TriggerProps = $props();

	const ctx = getContext<{ toggle: () => void }>('traceway-tooltip');

	function handleTouchStart(e: TouchEvent) {
		if (ctx) {
			e.preventDefault();
			ctx.toggle();
		}
	}
</script>

<TooltipPrimitive.Trigger
	bind:ref
	data-slot="tooltip-trigger"
	ontouchstart={handleTouchStart}
	{...restProps}
/>
