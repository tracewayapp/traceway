<script lang="ts" module>
	import { tv, type VariantProps } from "tailwind-variants";

	export const loadingCircleVariants = tv({
		base: "text-muted-foreground dark:text-white",
		variants: {
			size: {
				sm: "h-4 w-4",
				default: "h-6 w-6",
				md: "h-8 w-8",
				lg: "h-12 w-12",
				xlg: "h-18 w-18"
			}
		},
		defaultVariants: {
			size: "default"
		}
	});

	export type LoadingCircleSize = VariantProps<typeof loadingCircleVariants>["size"];
</script>

<script lang="ts">
	import { cn } from "$lib/utils.js";

	let {
		class: className,
		size = "default"
	}: {
		class?: string;
		size?: LoadingCircleSize;
	} = $props();
</script>

<svg
	xmlns="http://www.w3.org/2000/svg"
	viewBox="0 0 500 500"
	data-slot="loading-circle"
	class={cn(loadingCircleVariants({ size }), className)}
>
	<g fill="none" stroke="currentColor" stroke-linecap="round" stroke-width="31.25">
		<g class="outer-ring">
			<path
				d="M250 41.67c115.06 0 208.33 93.27 208.33 208.33c0 37.94-10.15 73.54-27.88 104.17M104.17 101.23A207.71 207.71 0 0 0 41.67 250c0 115.06 93.27 208.33 208.33 208.33c37.94 0 73.54-10.15 104.17-27.88"
			/>
		</g>
		<g class="middle-ring">
			<path
				d="M104.17 250c0 30.98 9.67 59.71 26.15 83.33M250 104.17a145.83 145.83 0 1 1-62.5 277.63"
			/>
		</g>
		<g class="inner-ring">
			<path d="M250 333.33a83.33 83.33 0 0 0 0-166.67" />
		</g>
	</g>
</svg>

<style>
	@keyframes spin-slow {
		from {
			transform: rotate(0deg);
		}
		to {
			transform: rotate(360deg);
		}
	}
	@keyframes spin-medium {
		from {
			transform: rotate(0deg);
		}
		to {
			transform: rotate(-360deg);
		}
	}
	@keyframes spin-fast {
		from {
			transform: rotate(0deg);
		}
		to {
			transform: rotate(360deg);
		}
	}
	.outer-ring {
		animation: spin-slow 3s linear infinite;
		transform-origin: 250px 250px;
	}
	.middle-ring {
		animation: spin-medium 2s linear infinite;
		transform-origin: 250px 250px;
	}
	.inner-ring {
		animation: spin-fast 1.2s linear infinite;
		transform-origin: 250px 250px;
	}
</style>
