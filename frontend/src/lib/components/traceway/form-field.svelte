<script lang="ts">
	import type { Snippet } from 'svelte';
	import { Label } from '$lib/components/ui/label';

	interface Props {
		label: string;
		for?: string;
		hint?: string;
		children: Snippet;
	}

	let { label, for: htmlFor, hint, children }: Props = $props();
</script>

<div class="space-y-2">
	<Label for={htmlFor}>{label}</Label>
	{@render children()}
	{#if hint}
		<p class="text-xs text-muted-foreground">{hint}</p>
	{/if}
</div>
