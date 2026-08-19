<script lang="ts">
	import CopyButton from '$lib/components/traceway/copy-button.svelte';

	let { value }: { value: string } = $props();
</script>

<div class="flex items-center gap-2">
	<code class="flex-1 rounded-md bg-muted px-3 py-2 font-mono text-sm break-all">{value}</code>
	<CopyButton text={value} />
</div>
