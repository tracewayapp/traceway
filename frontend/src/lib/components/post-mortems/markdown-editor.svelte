<script lang="ts">
	import { onMount } from 'svelte';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { themeState } from '$lib/state/theme.svelte';

	interface Props {
		value: string;
		readonly?: boolean;
	}

	let { value = $bindable(), readonly = false }: Props = $props();

	let container = $state<HTMLDivElement | null>(null);
	let ready = $state(false);
	let failed = $state(false);

	type CrepeInstance = {
		create: () => Promise<unknown>;
		destroy: () => void;
		getMarkdown: () => string;
		setReadonly: (value: boolean) => unknown;
		on: (fn: (api: { markdownUpdated: (fn: () => void) => void }) => void) => unknown;
	};
	let crepe: CrepeInstance | null = null;
	let themeStyle: HTMLStyleElement | null = null;
	let lightCss = '';
	let darkCss = '';

	onMount(() => {
		let destroyed = false;
		(async () => {
			try {
				const [{ Crepe }, common, light, dark] = await Promise.all([
					import('@milkdown/crepe'),
					import('@milkdown/crepe/theme/common/style.css?inline'),
					import('@milkdown/crepe/theme/frame.css?inline'),
					import('@milkdown/crepe/theme/frame-dark.css?inline')
				]);
				if (destroyed || !container) return;
				lightCss = light.default;
				darkCss = dark.default;

				const commonStyle = document.createElement('style');
				commonStyle.dataset.crepe = 'common';
				commonStyle.textContent = common.default;
				document.head.appendChild(commonStyle);
				themeStyle = document.createElement('style');
				themeStyle.dataset.crepe = 'theme';
				themeStyle.textContent = themeState.isDark ? darkCss : lightCss;
				document.head.appendChild(themeStyle);

				const instance = new Crepe({
					root: container,
					defaultValue: value,
					featureConfigs: {
						[Crepe.Feature.Placeholder]: {
							text: 'Write the post-mortem: what happened, why, and what changes...'
						}
					}
				}) as CrepeInstance;
				instance.on((api) => {
					api.markdownUpdated(() => {
						value = instance.getMarkdown();
					});
				});
				instance.setReadonly(readonly);
				await instance.create();
				if (destroyed) {
					instance.destroy();
					return;
				}
				crepe = instance;
				ready = true;
			} catch {
				if (!destroyed) failed = true;
			}
		})();
		return () => {
			destroyed = true;
			crepe?.destroy();
			crepe = null;
			for (const style of document.head.querySelectorAll('style[data-crepe]')) {
				style.remove();
			}
		};
	});

	$effect(() => {
		crepe?.setReadonly(readonly);
	});

	$effect(() => {
		if (themeStyle && ready) {
			themeStyle.textContent = themeState.isDark ? darkCss : lightCss;
		}
	});
</script>

{#if failed}
	<div class="rounded-md border py-16 text-center text-red-500">Failed to load the editor.</div>
{:else}
	<div class="relative min-h-[420px] rounded-md border">
		{#if !ready}
			<div class="absolute inset-0 flex items-center justify-center">
				<LoadingCircle size="xlg" />
			</div>
		{/if}
		<div bind:this={container} class="post-mortem-editor" class:invisible={!ready}></div>
	</div>
{/if}

<style>
	.post-mortem-editor :global(.milkdown) {
		--crepe-font-title: var(--font-body);
		--crepe-font-default: var(--font-body);
		--crepe-font-code: var(--font-code);
		--crepe-color-background: transparent;
		min-height: 420px;
	}
	.post-mortem-editor :global(.milkdown .ProseMirror) {
		min-height: 380px;
		padding: 1.25rem 1rem;
	}
	.post-mortem-editor :global(.milkdown .ProseMirror h1),
	.post-mortem-editor :global(.milkdown .ProseMirror h2),
	.post-mortem-editor :global(.milkdown .ProseMirror h3) {
		font-weight: 650;
		letter-spacing: -0.02em;
	}
	@media (min-width: 640px) {
		.post-mortem-editor :global(.milkdown .ProseMirror) {
			padding: 1.5rem 2rem;
		}
	}
</style>
