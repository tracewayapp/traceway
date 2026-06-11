<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Copy, Check } from 'lucide-svelte';
	import Highlight from 'svelte-highlight';
	import type { LanguageFn } from 'highlight.js';
	import { themeState } from '$lib/state/theme.svelte';
	import 'svelte-highlight/styles/github-dark.css';

	let {
		code,
		language,
		wrap = false
	}: { code: string; language: { name: string; register: LanguageFn }; wrap?: boolean } = $props();

	let copied = $state(false);

	async function copy() {
		await navigator.clipboard.writeText(code);
		copied = true;
		setTimeout(() => (copied = false), 2000);
	}
</script>

<div class="relative">
	<div class="absolute top-2 right-2 z-10">
		<Button variant="outline" size="sm" onclick={copy}>
			{#if copied}
				<Check class="mr-2 h-4 w-4 text-green-500" />
				Copied!
			{:else}
				<Copy class="mr-2 h-4 w-4" />
				Copy
			{/if}
		</Button>
	</div>
	<div
		class="overflow-x-auto rounded-lg text-sm {wrap ? 'wrap-code' : ''} {themeState.isDark
			? 'dark-code'
			: 'light-code'}"
	>
		<Highlight {language} {code} />
	</div>
</div>

<style>
	:global(.wrap-code pre code) {
		white-space: pre-wrap;
		word-break: break-word;
	}

	:global(.light-code .hljs-name) {
		color: #4ba3f7;
	}
	:global(.light-code .hljs) {
		background: #f6f8fa;
		color: #24292e;
	}
	:global(.light-code .hljs-keyword),
	:global(.light-code .hljs-selector-tag) {
		color: #d73a49;
	}
	:global(.light-code .hljs-string),
	:global(.light-code .hljs-attr) {
		color: #032f62;
	}
	:global(.light-code .hljs-function),
	:global(.light-code .hljs-title) {
		color: #6f42c1;
	}
	:global(.light-code .hljs-comment) {
		color: #6a737d;
	}
	:global(.light-code .hljs-built_in) {
		color: #005cc5;
	}
	:global(.light-code .hljs-meta) {
		color: #d73a49;
	}
	:global(.light-code .hljs-variable) {
		color: #24292e;
	}

	:global(.dark-code .hljs) {
		background: #0d1117;
		color: #c9d1d9;
	}
	:global(.dark-code .hljs-keyword),
	:global(.dark-code .hljs-selector-tag) {
		color: #ff7b72;
	}
	:global(.dark-code .hljs-string),
	:global(.dark-code .hljs-attr) {
		color: #a5d6ff;
	}
	:global(.dark-code .hljs-function),
	:global(.dark-code .hljs-title) {
		color: #d2a8ff;
	}
	:global(.dark-code .hljs-comment) {
		color: #8b949e;
	}
	:global(.dark-code .hljs-built_in) {
		color: #79c0ff;
	}
	:global(.dark-code .hljs-meta) {
		color: #ff7b72;
	}
</style>
