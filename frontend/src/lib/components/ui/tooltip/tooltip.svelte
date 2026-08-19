<script lang="ts">
	import { Tooltip as TooltipPrimitive } from 'bits-ui';
	import { setContext } from 'svelte';

	let { open = $bindable(false), ...restProps }: TooltipPrimitive.RootProps = $props();

	let openedByTouch = false;
	let removeListener: (() => void) | null = null;

	function closeTooltip() {
		openedByTouch = false;
		open = false;
		if (removeListener) {
			removeListener();
			removeListener = null;
		}
	}

	function toggle() {
		if (open && openedByTouch) {
			closeTooltip();
			return;
		}
		openedByTouch = true;
		open = true;
		setTimeout(() => {
			const handler = () => closeTooltip();
			document.addEventListener('pointerdown', handler);
			removeListener = () => document.removeEventListener('pointerdown', handler);
		}, 0);
	}

	function handleOpenChange(v: boolean) {
		if (!v && openedByTouch) {
			open = true;
			return;
		}
		open = v;
	}

	setContext('traceway-tooltip', { toggle });
</script>

<TooltipPrimitive.Root bind:open {...restProps} onOpenChange={handleOpenChange} />
