<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Copy, Check } from 'lucide-svelte';

	let { value }: { value: string } = $props();

	let copied = $state(false);

	async function copy() {
		await navigator.clipboard.writeText(value);
		copied = true;
		setTimeout(() => (copied = false), 2000);
	}
</script>

<div class="flex items-center gap-2">
	<code class="flex-1 rounded-md bg-muted px-3 py-2 font-mono text-sm break-all">{value}</code>
	<Button variant="outline" size="sm" onclick={copy}>
		{#if copied}
			<Check class="h-4 w-4 text-green-500" />
		{:else}
			<Copy class="h-4 w-4" />
		{/if}
	</Button>
</div>
