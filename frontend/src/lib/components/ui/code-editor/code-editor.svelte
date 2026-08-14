<script lang="ts">
	import { onMount } from 'svelte';
	import { EditorView, basicSetup } from 'codemirror';
	import { EditorState, Compartment } from '@codemirror/state';
	import { javascript } from '@codemirror/lang-javascript';
	import { oneDark } from '@codemirror/theme-one-dark';
	import { themeState } from '$lib/state/theme.svelte';
	import { cn } from '$lib/utils';

	interface Props {
		value: string;
		readOnly?: boolean;
		minHeight?: string;
		class?: string;
	}

	let {
		value = $bindable(),
		readOnly = false,
		minHeight = '280px',
		class: className = ''
	}: Props = $props();

	let container: HTMLDivElement;
	let view: EditorView | null = null;

	const themeCompartment = new Compartment();
	const readOnlyCompartment = new Compartment();

	const baseTheme = EditorView.theme({
		'&': { fontSize: '12px' },
		// minHeight lives on the scroller: it is the flex row holding gutters +
		// content, so both stretch to fill the box instead of stopping at the
		// last line.
		'.cm-scroller': { minHeight },
		'.cm-content': { fontFamily: 'var(--font-mono, monospace)' },
		'.cm-gutters': { fontFamily: 'var(--font-mono, monospace)' },
		'&.cm-focused': { outline: 'none' }
	});

	const lightTheme = EditorView.theme({}, { dark: false });

	onMount(() => {
		view = new EditorView({
			parent: container,
			state: EditorState.create({
				doc: value,
				extensions: [
					basicSetup,
					javascript({ typescript: true }),
					baseTheme,
					themeCompartment.of(themeState.isDark ? oneDark : lightTheme),
					readOnlyCompartment.of(EditorState.readOnly.of(readOnly)),
					EditorView.updateListener.of((update) => {
						if (update.docChanged) {
							value = update.state.doc.toString();
						}
					})
				]
			})
		});
		return () => {
			view?.destroy();
			view = null;
		};
	});

	// External value changes (populating the form for editing).
	$effect(() => {
		const next = value;
		if (view && next !== view.state.doc.toString()) {
			view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: next } });
		}
	});

	$effect(() => {
		const dark = themeState.isDark;
		view?.dispatch({ effects: themeCompartment.reconfigure(dark ? oneDark : lightTheme) });
	});

	$effect(() => {
		const ro = readOnly;
		view?.dispatch({ effects: readOnlyCompartment.reconfigure(EditorState.readOnly.of(ro)) });
	});
</script>

<div
	bind:this={container}
	class={cn('overflow-hidden rounded-md border border-input text-left shadow-xs', className)}
></div>
