<script lang="ts">
	import JSONTree from 'svelte-json-tree';
	import { Copy, Check } from 'lucide-svelte';

	let {
		title,
		value
	}: {
		title: string;
		value: string;
	} = $props();

	let copied = $state(false);

	const parsedJson = $derived(() => {
		try {
			const parsed = JSON.parse(value);
			if (typeof parsed === 'object' && parsed !== null) {
				return parsed;
			}
		} catch {
			// Not valid JSON
		}
		return null;
	});

	async function copyToClipboard() {
		await navigator.clipboard.writeText(value);
		copied = true;
		setTimeout(() => {
			copied = false;
		}, 2000);
	}
</script>

<div class="group relative flex flex-col gap-1 rounded-md bg-muted p-3">
	<div class="flex items-center justify-between">
		<span class="text-xs font-medium text-muted-foreground">{title}</span>
		<button
			onclick={copyToClipboard}
			class="opacity-0 group-hover:opacity-100 transition-opacity text-muted-foreground hover:text-foreground"
			title="Copy to clipboard"
		>
			{#if copied}
				<Check class="h-3.5 w-3.5 text-green-500" />
			{:else}
				<Copy class="h-3.5 w-3.5" />
			{/if}
		</button>
	</div>
	{#if parsedJson()}
		<div class="json-tree-container">
			<JSONTree value={parsedJson()} />
		</div>
	{:else}
		<span class="font-mono text-sm break-all">{value}</span>
	{/if}
</div>

<style>
	.json-tree-container {
		--json-tree-font-size: 0.875rem;
		--json-tree-font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
		--json-tree-li-indentation: 1.25em;
		--json-tree-li-line-height: 1.5;

		/* Light mode colors */
		--json-tree-string-color: oklch(0.6 0.15 25);
		--json-tree-symbol-color: oklch(0.6 0.15 25);
		--json-tree-boolean-color: oklch(0.5 0.15 260);
		--json-tree-function-color: oklch(0.5 0.15 260);
		--json-tree-number-color: oklch(0.5 0.18 280);
		--json-tree-label-color: oklch(0.5 0.15 320);
		--json-tree-property-color: oklch(0.35 0.05 265);
		--json-tree-arrow-color: oklch(0.55 0.03 265);
		--json-tree-operator-color: oklch(0.55 0.03 265);
		--json-tree-null-color: oklch(0.55 0.03 265);
		--json-tree-undefined-color: oklch(0.55 0.03 265);
		--json-tree-date-color: oklch(0.55 0.03 265);
		--json-tree-internal-color: oklch(0.55 0.03 265);
		--json-tree-regex-color: oklch(0.6 0.15 25);
	}

	:global(.dark) .json-tree-container {
		/* Dark mode colors */
		--json-tree-string-color: oklch(0.75 0.15 25);
		--json-tree-symbol-color: oklch(0.75 0.15 25);
		--json-tree-boolean-color: oklch(0.7 0.15 260);
		--json-tree-function-color: oklch(0.7 0.15 260);
		--json-tree-number-color: oklch(0.7 0.18 280);
		--json-tree-label-color: oklch(0.7 0.15 320);
		--json-tree-property-color: oklch(0.9 0.02 265);
		--json-tree-arrow-color: oklch(0.65 0 0);
		--json-tree-operator-color: oklch(0.65 0 0);
		--json-tree-null-color: oklch(0.65 0 0);
		--json-tree-undefined-color: oklch(0.65 0 0);
		--json-tree-date-color: oklch(0.65 0 0);
		--json-tree-internal-color: oklch(0.65 0 0);
		--json-tree-regex-color: oklch(0.75 0.15 25);
	}
</style>
