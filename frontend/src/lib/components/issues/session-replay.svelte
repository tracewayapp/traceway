<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { Globe, Play, Pause, Maximize, Minimize } from 'lucide-svelte';
	import type { eventWithTime } from '@rrweb/types';
	// Static side-effect import — keeps the rrweb-player stylesheet attached
	// to the page for the lifetime of the SPA, even across mount/unmount cycles.
	// A dynamic import inside onMount would let Vite's HMR dispose the style
	// node when the component unmounts, leaving the next mount with no
	// background fill on .replayer-wrapper until a hard refresh.
	import 'rrweb-player/dist/style.css';

	interface FlutterVideoEvent {
		type: 'flutter_video';
		data: string;
		format: string;
		fps: number;
		durationSeconds: number;
	}

	interface Props {
		events: (eventWithTime & { data?: { href?: string } })[] | FlutterVideoEvent[] | null;
		onTimeUpdate?: (ms: number) => void;
	}

	let { events, onTimeUpdate }: Props = $props();
	let container: HTMLElement;
	let player: any = null;
	let resizeObserver: ResizeObserver | null = null;
	let rafId: number | null = null;
	let lastReportedMs = -1;

	export function seek(offsetMs: number) {
		const ms = Math.max(0, offsetMs);
		if (isFlutterVideo && videoEl) {
			videoEl.currentTime = ms / 1000;
		} else if (player) {
			player.goto(ms, false);
		}
	}

	let isFlutterVideo = $derived(
		events && events.length > 0 && (events[0] as any)?.type === 'flutter_video'
	);

	let flutterVideoSrc = $derived.by(() => {
		if (!isFlutterVideo || !events) return '';
		const evt = events[0] as unknown as FlutterVideoEvent;
		return `data:video/mp4;base64,${evt.data}`;
	});

	let videoEl: HTMLVideoElement;
	let videoContainerEl: HTMLElement;
	let playing = $state(false);
	let currentTime = $state(0);
	let duration = $state(0);
	let playbackSpeed = $state(1);
	let isFullscreen = $state(false);
	let dragging = $state(false);
	let progressPercent = $derived(duration > 0 ? (currentTime / duration) * 100 : 0);

	function formatTime(seconds: number): string {
		const m = Math.floor(seconds / 60);
		const s = Math.floor(seconds % 60);
		return `${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`;
	}

	function togglePlay() {
		if (!videoEl) return;
		if (videoEl.paused) {
			if (videoEl.ended || videoEl.currentTime >= videoEl.duration - 0.1) {
				videoEl.currentTime = 0;
			}
			videoEl.play();
		} else {
			videoEl.pause();
		}
	}

	function setSpeed(speed: number) {
		playbackSpeed = speed;
		if (videoEl) videoEl.playbackRate = speed;
	}

	function toggleFullscreen() {
		if (!videoContainerEl) return;
		if (document.fullscreenElement) {
			document.exitFullscreen();
		} else {
			videoContainerEl.requestFullscreen();
		}
	}

	function handleProgressClick(e: MouseEvent) {
		if (!videoEl || !duration) return;
		const target = e.currentTarget as HTMLElement;
		const rect = target.getBoundingClientRect();
		const ratio = Math.max(0, Math.min(1, (e.clientX - rect.left) / rect.width));
		videoEl.currentTime = ratio * duration;
	}

	function handleProgressDragStart(e: MouseEvent) {
		e.preventDefault();
		dragging = true;
		const progressEl = (e.currentTarget as HTMLElement).parentElement!;

		const onMove = (ev: MouseEvent) => {
			const rect = progressEl.getBoundingClientRect();
			const ratio = Math.max(0, Math.min(1, (ev.clientX - rect.left) / rect.width));
			currentTime = ratio * duration;
			if (videoEl) videoEl.currentTime = currentTime;
		};

		const onUp = () => {
			dragging = false;
			window.removeEventListener('mousemove', onMove);
			window.removeEventListener('mouseup', onUp);
		};

		window.addEventListener('mousemove', onMove);
		window.addEventListener('mouseup', onUp);
	}

	$effect(() => {
		if (!videoEl) return;

		const onVideoTimeUpdate = () => {
			if (!dragging) currentTime = videoEl.currentTime;
			onTimeUpdate?.(videoEl.currentTime * 1000);
		};
		const onLoadedMetadata = () => {
			duration = videoEl.duration;
			videoEl.currentTime = videoEl.duration;
		};
		const onPlay = () => {
			playing = true;
		};
		const onPause = () => {
			playing = false;
		};
		const onFullscreenChange = () => {
			isFullscreen = !!document.fullscreenElement;
		};

		videoEl.addEventListener('timeupdate', onVideoTimeUpdate);
		videoEl.addEventListener('loadedmetadata', onLoadedMetadata);
		videoEl.addEventListener('play', onPlay);
		videoEl.addEventListener('pause', onPause);
		document.addEventListener('fullscreenchange', onFullscreenChange);

		return () => {
			videoEl.removeEventListener('timeupdate', onVideoTimeUpdate);
			videoEl.removeEventListener('loadedmetadata', onLoadedMetadata);
			videoEl.removeEventListener('play', onPlay);
			videoEl.removeEventListener('pause', onPause);
			document.removeEventListener('fullscreenchange', onFullscreenChange);
		};
	});

	let pageUrl = $derived.by(() => {
		if (!events || events.length === 0 || isFlutterVideo) return '';
		const metaEvent = events.find((e: any) => e.type === 4);
		return metaEvent?.data?.href ?? '';
	});

	function getPlayerHeight(width: number): number {
		return Math.round((width * 9) / 16);
	}

	onMount(async () => {
		if (!events || events.length === 0 || isFlutterVideo) return;

		const { default: rrwebPlayer } = await import('rrweb-player');

		const width = Math.round(container.clientWidth * 0.75);

		const chartColor = getComputedStyle(container).getPropertyValue('--chart-1').trim();

		player = new rrwebPlayer({
			target: container,
			props: {
				events,
				width,
				height: getPlayerHeight(width),
				autoPlay: false,
				mouseTail: {
					strokeStyle: chartColor
				}
			}
		});

		const firstTs = (events[0] as eventWithTime).timestamp;
		const lastTs = (events[events.length - 1] as eventWithTime).timestamp;
		player.goto(lastTs - firstTs, true);

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

		const pollTime = () => {
			if (!player) return;
			try {
				const t = player.getReplayer().getCurrentTime();
				if (t !== lastReportedMs) {
					lastReportedMs = t;
					onTimeUpdate?.(t);
				}
			} catch {
				// Replayer may briefly be unavailable during init/destroy
			}
			rafId = requestAnimationFrame(pollTime);
		};
		rafId = requestAnimationFrame(pollTime);
	});

	onDestroy(() => {
		if (rafId !== null) cancelAnimationFrame(rafId);
		resizeObserver?.disconnect();
		if (player) player.$destroy();
	});
</script>

{#if events && events.length > 0}
	<div class="player-wrapper overflow-hidden">
		{#if isFlutterVideo}
			<div class="url-bar">
				<img src="/flutter.png" alt="Flutter" class="flutter-icon" />
				<span class="url-bar-text">Flutter Screen Recording</span>
			</div>
			<div class="flutter-video-container" bind:this={videoContainerEl}>
				<video
					bind:this={videoEl}
					src={flutterVideoSrc}
					playsinline
					onclick={togglePlay}
				>
					<track kind="captions" />
				</video>
				<div class="fv-controller">
					<div class="fv-timeline">
						<span class="fv-timeline__time">{formatTime(currentTime)}</span>
						<!-- svelte-ignore a11y_no_static_element_interactions -->
						<div
							class="fv-progress"
							role="slider"
							tabindex="0"
							aria-valuemin={0}
							aria-valuemax={duration}
							aria-valuenow={currentTime}
							onclick={handleProgressClick}
							onkeydown={(e) => {
								if (!videoEl) return;
								if (e.key === 'ArrowRight') videoEl.currentTime = Math.min(duration, currentTime + 5);
								if (e.key === 'ArrowLeft') videoEl.currentTime = Math.max(0, currentTime - 5);
							}}
						>
							<div class="fv-progress__step" style:width="{progressPercent}%"></div>
							<div
								class="fv-progress__handler"
								style:left="{progressPercent}%"
								onmousedown={handleProgressDragStart}
							></div>
						</div>
						<span class="fv-timeline__time">{formatTime(duration)}</span>
					</div>
					<div class="fv-controller__btns">
						<button onclick={togglePlay}>
							{#if playing}
								<Pause size={16} />
							{:else}
								<Play size={16} />
							{/if}
						</button>
						{#each [1, 2, 4, 8] as s}
							<button
								class:active={playbackSpeed === s}
								onclick={() => setSpeed(s)}
							>
								{s}x
							</button>
						{/each}
						<button onclick={toggleFullscreen}>
							{#if isFullscreen}
								<Minimize size={16} />
							{:else}
								<Maximize size={16} />
							{/if}
						</button>
					</div>
				</div>
			</div>
		{:else}
			{#if pageUrl}
				<div class="url-bar">
					<Globe />
					<span class="url-bar-text">{pageUrl}</span>
				</div>
			{/if}
			<div bind:this={container} class="player-container"></div>
		{/if}
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

	.flutter-icon {
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
		border-top: none !important;
		border-bottom: none !important;
	}

	/* Progress fill */
	.player-wrapper :global(.rr-progress__step) {
		background: var(--chart-1) !important;
		border-radius: 3px !important;
	}

	/* Inactive-period markers */
	.player-wrapper :global(.rr-progress > div:not(.rr-progress__step):not(.rr-progress__handler)) {
		display: none !important;
	}

	/* Progress handle */
	.player-wrapper :global(.rr-progress__handler) {
		width: 14px !important;
		height: 14px !important;
		background: var(--chart-1) !important;
		border-radius: 50% !important;
		top: 50% !important;
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

	/* Flutter video player */
	.flutter-video-container {
		display: flex;
		flex-direction: column;
		align-items: center;
		background: var(--muted);
		min-height: 300px;
	}

	.flutter-video-container video {
		max-width: 100%;
		max-height: 70vh;
		background: #000;
		cursor: pointer;
		flex: 1;
	}

	.flutter-video-container:fullscreen {
		background: #000;
		justify-content: center;
	}

	.flutter-video-container:fullscreen video {
		max-height: calc(100vh - 80px);
	}

	.flutter-video-container:fullscreen .fv-controller {
		position: absolute;
		bottom: 0;
		left: 0;
		right: 0;
		opacity: 0;
		transition: opacity 0.2s;
	}

	.flutter-video-container:fullscreen:hover .fv-controller {
		opacity: 1;
	}

	.fv-controller {
		width: 100%;
		background: var(--muted);
		border-top: 1px solid var(--border);
		border-radius: 0px 0px 9px 9px;
		display: flex;
		flex-direction: column;
		justify-content: space-around;
		align-items: center;
		padding: 8px 0;
	}

	.fv-timeline {
		width: 80%;
		display: flex;
		align-items: center;
		padding: 8px 0;
	}

	.fv-timeline__time {
		display: inline-block;
		width: 80px;
		text-align: center;
		font-family: ui-monospace, SFMono-Regular, 'SF Mono', Menlo, Consolas, monospace;
		font-size: 12px;
		color: var(--muted-foreground);
	}

	.fv-progress {
		flex: 1;
		height: 6px;
		background: var(--border);
		position: relative;
		border-radius: 3px;
		cursor: pointer;
	}

	.fv-progress__step {
		height: 100%;
		position: absolute;
		left: 0;
		top: 0;
		background: var(--chart-1);
		border-radius: 3px;
	}

	.fv-progress__handler {
		width: 14px;
		height: 14px;
		background: var(--chart-1);
		border-radius: 50%;
		position: absolute;
		top: 50%;
		transform: translate(-50%, -50%);
		cursor: grab;
	}

	.fv-progress__handler:active {
		cursor: grabbing;
	}

	.fv-controller__btns {
		display: flex;
		align-items: center;
		justify-content: center;
		font-size: 13px;
		gap: 2px;
	}

	.fv-controller__btns button {
		width: 32px;
		height: 32px;
		display: flex;
		padding: 0;
		align-items: center;
		justify-content: center;
		background: none;
		border: none;
		border-radius: 50%;
		cursor: pointer;
		color: var(--foreground);
	}

	.fv-controller__btns button:hover {
		background: var(--accent);
	}

	.fv-controller__btns button.active {
		background: var(--chart-1);
		color: #fff;
	}

	.fv-controller__btns button:active {
		background: var(--chart-1);
		opacity: 0.8;
	}
</style>
