<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Copy, Check } from 'lucide-svelte';
	import type { PromptPart } from '$lib/utils/ai-setup';

	let { parts }: { parts: PromptPart[] } = $props();

	const text = $derived(parts.map((p) => p.text).join(''));

	let copied = $state(false);

	async function copy() {
		await navigator.clipboard.writeText(text);
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
	<code
		class="block rounded-lg bg-muted py-3 pr-24 pl-4 font-mono text-sm break-words whitespace-pre-wrap text-foreground"
	>
		{#each parts as part, i (i)}
			{#if part.bold}
				<span class="break-all text-muted-foreground">{part.text}</span>
			{:else}
				{part.text}
			{/if}
		{/each}
	</code>
</div>
