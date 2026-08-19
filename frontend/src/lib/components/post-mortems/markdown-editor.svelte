<script lang="ts">
	import { onMount } from 'svelte';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import * as Tabs from '$lib/components/ui/tabs';
	import { themeState } from '$lib/state/theme.svelte';

	interface Props {
		value: string;
		readonly?: boolean;
	}

	let { value = $bindable(), readonly = false }: Props = $props();

	let container = $state<HTMLDivElement | null>(null);
	let mode = $state<'rich' | 'markdown'>('rich');
	let ready = $state(false);
	let failed = $state(false);

	type CrepeInstance = {
		create: () => Promise<unknown>;
		destroy: () => void;
		getMarkdown: () => string;
		setReadonly: (value: boolean) => unknown;
		on: (fn: (api: { markdownUpdated: (fn: () => void) => void }) => void) => unknown;
	};
	type CrepeCtor = {
		new (config: {
			root: HTMLElement;
			defaultValue: string;
			featureConfigs: Record<string, unknown>;
		}): CrepeInstance;
		Feature: { Placeholder: string };
	};
	let CrepeClass: CrepeCtor | null = null;
	let crepe: CrepeInstance | null = null;
	let themeStyle: HTMLStyleElement | null = null;
	let lightCss = '';
	let darkCss = '';
	let destroyed = false;
	let createSeq = 0;

	async function ensureAssets(): Promise<boolean> {
		if (CrepeClass) return true;
		try {
			const [{ Crepe }, common, light, dark] = await Promise.all([
				import('@milkdown/crepe'),
				import('@milkdown/crepe/theme/common/style.css?inline'),
				import('@milkdown/crepe/theme/frame.css?inline'),
				import('@milkdown/crepe/theme/frame-dark.css?inline')
			]);
			if (destroyed) return false;
			CrepeClass = Crepe as unknown as CrepeCtor;
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
			return true;
		} catch {
			return false;
		}
	}

	async function showRich() {
		const seq = ++createSeq;
		failed = false;
		const loaded = await ensureAssets();
		if (destroyed || seq !== createSeq) return;
		if (!loaded || !CrepeClass || !container) {
			failed = true;
			return;
		}
		try {
			const instance = new CrepeClass({
				root: container,
				defaultValue: value,
				featureConfigs: {
					[CrepeClass.Feature.Placeholder]: {
						text: 'Write the post-mortem: what happened, why, and what changes. Type / to add headings, lists, tables...'
					}
				}
			});
			instance.on((api) => {
				api.markdownUpdated(() => {
					value = instance.getMarkdown();
				});
			});
			instance.setReadonly(readonly);
			await instance.create();
			if (destroyed || seq !== createSeq) {
				instance.destroy();
				return;
			}
			crepe = instance;
			ready = true;
		} catch {
			if (!destroyed && seq === createSeq) failed = true;
		}
	}

	function destroyEditor() {
		createSeq++;
		crepe?.destroy();
		crepe = null;
		ready = false;
	}

	function setMode(next: string) {
		if ((next !== 'rich' && next !== 'markdown') || next === mode) return;
		mode = next;
		if (next === 'markdown') {
			// The markdownUpdated listener is debounced, so pull the latest content
			// directly before tearing the instance down.
			if (crepe && ready) {
				try {
					value = crepe.getMarkdown();
				} catch {
					// keep the last synced value
				}
			}
			destroyEditor();
			failed = false;
		} else {
			showRich();
		}
	}

	onMount(() => {
		showRich();
		return () => {
			destroyed = true;
			crepe?.destroy();
			crepe = null;
			for (const style of document.head.querySelectorAll('style[data-crepe]')) {
				style.remove();
			}
		};
	});

	// Read the reactive values unconditionally: `crepe`/`themeStyle` are plain
	// variables, and short-circuiting past the reactive reads while they are
	// still null would leave these effects with no dependencies to re-run on.
	$effect(() => {
		const ro = readonly;
		crepe?.setReadonly(ro);
	});

	$effect(() => {
		const isDark = themeState.isDark;
		if (!ready || !themeStyle) return;
		themeStyle.textContent = isDark ? darkCss : lightCss;
	});
</script>

<div class="space-y-2">
	<div class="flex justify-end">
		<Tabs.Root value={mode} onValueChange={setMode}>
			<Tabs.List class="h-8">
				<Tabs.Trigger value="rich" class="px-2.5 py-0.5 text-xs">Rich text</Tabs.Trigger>
				<Tabs.Trigger value="markdown" class="px-2.5 py-0.5 text-xs">Markdown</Tabs.Trigger>
			</Tabs.List>
		</Tabs.Root>
	</div>

	<div class="relative min-h-[420px] rounded-md border">
		{#if mode === 'rich' && failed}
			<div class="flex min-h-[420px] items-center justify-center px-4 text-center text-sm text-red-500">
				Failed to load the editor. Switch to Markdown to keep editing.
			</div>
		{:else if mode === 'rich' && !ready}
			<div class="absolute inset-0 flex items-center justify-center">
				<LoadingCircle size="xlg" />
			</div>
		{/if}
		{#if mode === 'markdown'}
			<textarea
				bind:value
				{readonly}
				spellcheck={false}
				placeholder="Write the post-mortem in Markdown..."
				class="post-mortem-raw block min-h-[420px] w-full resize-y rounded-md bg-transparent px-4 py-5 text-sm outline-none placeholder:text-muted-foreground sm:px-8 sm:py-6"
			></textarea>
		{/if}
		<div
			bind:this={container}
			class="post-mortem-editor"
			class:hidden={mode !== 'rich' || failed}
			class:invisible={!ready}
		></div>
	</div>
</div>

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
		padding: 0.875rem 1rem 1.25rem;
	}
	/* Headings carry a large margin-top for spacing between sections; on the
	   first block it just pushes the content away from the top border. */
	.post-mortem-editor :global(.milkdown .ProseMirror > *:first-child) {
		margin-top: 0;
	}
	.post-mortem-editor :global(.milkdown .ProseMirror h1),
	.post-mortem-editor :global(.milkdown .ProseMirror h2),
	.post-mortem-editor :global(.milkdown .ProseMirror h3) {
		font-weight: 650;
		letter-spacing: -0.02em;
	}
	@media (min-width: 640px) {
		.post-mortem-editor :global(.milkdown .ProseMirror) {
			padding: 1rem 2rem 1.5rem;
		}
	}
	.post-mortem-editor :global(.milkdown .milkdown-toolbar),
	.post-mortem-editor :global(.milkdown .milkdown-slash-menu),
	.post-mortem-editor :global(.milkdown .milkdown-link-preview),
	.post-mortem-editor :global(.milkdown .milkdown-link-edit) {
		box-shadow: none;
		border: 1px solid var(--border);
	}
	/* Formatting goes through the selection toolbar and the / menu; the floating
	   gutter handle (drag grip + plus) is hidden entirely. */
	.post-mortem-editor :global(.milkdown .milkdown-block-handle) {
		display: none;
	}
	.post-mortem-raw {
		font-family: var(--font-code);
	}
</style>
