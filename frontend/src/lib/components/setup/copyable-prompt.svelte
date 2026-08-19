<script lang="ts">
	import CopyButton from '$lib/components/traceway/copy-button.svelte';
	import type { PromptPart } from '$lib/utils/ai-setup';

	let { parts }: { parts: PromptPart[] } = $props();

	const text = $derived(parts.map((p) => p.text).join(''));
</script>

<div class="flex items-start gap-2">
	<code
		class="block min-w-0 flex-1 rounded-md bg-muted px-4 py-3 font-mono text-sm break-words whitespace-pre-wrap text-foreground"
	>
		{#each parts as part, i (i)}
			{#if part.bold}
				<span class="break-all text-muted-foreground">{part.text}</span>
			{:else}
				{part.text}
			{/if}
		{/each}
	</code>
	<CopyButton {text} label="Copy" />
</div>
