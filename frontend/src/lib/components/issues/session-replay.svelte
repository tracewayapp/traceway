<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { Globe } from 'lucide-svelte';
	import type { eventWithTime } from '@rrweb/types';

	interface Props {
		events: (eventWithTime & { data?: { href?: string } })[] | null;
	}

	let { events }: Props = $props();
	let container: HTMLElement;
	let player: any = null;
	let resizeObserver: ResizeObserver | null = null;

	let pageUrl = $derived.by(() => {
		if (!events || events.length === 0) return '';
		const metaEvent = events.find((e: any) => e.type === 4);
		return metaEvent?.data?.href ?? '';
	});

	function getPlayerHeight(width: number): number {
		return Math.round((width * 9) / 16);
	}

	onMount(async () => {
		if (!events || events.length === 0) return;

		const { default: rrwebPlayer } = await import('rrweb-player');
		await import('rrweb-player/dist/style.css');

		const width = Math.round(container.clientWidth * 0.75);

		const chartColor = getComputedStyle(container).getPropertyValue('--chart-1').trim();

		player = new rrwebPlayer({
			target: container,
			props: {
				events,
				width,
				height: getPlayerHeight(width),
				mouseTail: {
					strokeStyle: chartColor
				}
			}
		});

		resizeObserver = new ResizeObserver((entries) => {
			for (const entry of entries) {
				const newWidth = Math.round(entry.contentRect.width * 0.75);
				if (player && newWidth > 0) {
					player.$set({ width: newWidth, height: getPlayerHeight(newWidth) });
					player.triggerResize();
				}
			}
		});
		resizeObserver.observe(container);
	});

	onDestroy(() => {
		resizeObserver?.disconnect();
		if (player) player.$destroy();
	});
</script>

{#if events && events.length > 0}
	<div class="player-wrapper overflow-hidden">
		{#if pageUrl}
			<div class="url-bar">
				<Globe />
				<span class="url-bar-text">{pageUrl}</span>
			</div>
		{/if}
		<div bind:this={container} class="player-container"></div>
	</div>
{/if}

<style>
	.url-bar {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 8px 12px;
		border-bottom: 1px solid var(--border);
		border-top: 1px solid var(--border);
		color: var(--muted-foreground);
	}

	.url-bar :global(svg) {
		width: 14px;
		height: 14px;
		flex-shrink: 0;
	}

	.url-bar-text {
		font-family: ui-monospace, SFMono-Regular, 'SF Mono', Menlo, Consolas, monospace;
		font-size: 12px;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.player-container {
		width: 100%;
	}

	/* rrweb player root */
	.player-wrapper :global(.rr-player) {
		width: 100% !important;
		height: auto !important;
		background: transparent !important;
		box-shadow: none !important;
		border-radius: 0 !important;
	}

	.player-wrapper :global(.rr-player__frame) {
		margin: 0 auto !important;
	}

	/* Controller bar */
	.player-wrapper :global(.rr-controller) {
		background: var(--muted) !important;
		border-top: 1px solid var(--border) !important;
		border-radius: 0px 0px 9px 9px !important;
	}

	/* Timeline */
	.player-wrapper :global(.rr-timeline) {
		width: 100% !important;
	}

	.player-wrapper :global(.rr-timeline__time) {
		font-family: ui-monospace, SFMono-Regular, 'SF Mono', Menlo, Consolas, monospace;
		font-size: 12px !important;
		color: var(--muted-foreground) !important;
	}

	/* Progress track */
	.player-wrapper :global(.rr-progress) {
		height: 6px !important;
		background: var(--border) !important;
		border-radius: 3px !important;
	}

	/* Progress fill */
	.player-wrapper :global(.rr-progress__step) {
		background: var(--chart-1) !important;
		border-radius: 3px !important;
	}

	/* Progress handle */
	.player-wrapper :global(.rr-progress__handler) {
		width: 14px !important;
		height: 14px !important;
		background: var(--chart-1) !important;
		border-radius: 50% !important;
		top: -0px !important;
	}

	/* Buttons */
	.player-wrapper :global(.rr-controller__btns button) {
		color: var(--foreground) !important;
	}

	.player-wrapper :global(.rr-controller__btns button svg) {
		fill: currentColor;
	}

	.player-wrapper :global(.rr-controller__btns button:hover) {
		background: var(--accent) !important;
	}

	.player-wrapper :global(.rr-controller__btns button.active) {
		background: var(--chart-1) !important;
	}

	.player-wrapper :global(.rr-controller__btns button:active) {
		background: var(--chart-1) !important;
		opacity: 0.8;
	}

	/* Switch toggle */
	.player-wrapper :global(.switch label:before) {
		background: var(--border) !important;
	}

	.player-wrapper :global(.switch input[type='checkbox']:checked + label:before) {
		background: var(--chart-1) !important;
	}

	/* Replayer wrapper */
	.player-wrapper :global(.replayer-wrapper) {
		border-color: transparent !important;
	}

	.player-wrapper :global(.replayer-wrapper.touch-active) {
		border-color: var(--chart-1) !important;
	}

	/* Mouse cursor */
	.player-wrapper :global(.replayer-mouse) {
		background-image: url('data:image/svg+xml;charset=utf-8;base64,PHN2ZyB4bWxucz0naHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmcnIHdpZHRoPSczMDBweCcgaGVpZ2h0PSczMDBweCcgdmlld0JveD0nMCAwIDUwIDUwJz48cGF0aCBkPSdNNSAyTDUgNDNMMTQuNSAzM0wyMyA0OEwzMCA0NC41TDIxLjUgMjkuNUwzNSAyOS41WicgZmlsbD0nIzAwMDAwMCcgc3Ryb2tlPScjZmZmZmZmJyBzdHJva2Utd2lkdGg9JzIuNScgc3Ryb2tlLWxpbmVqb2luPSdyb3VuZCcvPjwvc3ZnPg==') !important;
	}
</style>
