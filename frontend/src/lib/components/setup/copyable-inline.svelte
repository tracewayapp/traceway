<script lang="ts">
	import CopyButton from '$lib/components/traceway/copy-button.svelte';
	import { cn } from '$lib/utils';
	import { REPLAY_BLOCK_CLASS } from '$lib/utils/self-monitoring';

	let { value, secret = false }: { value: string; secret?: boolean } = $props();
</script>

<div class="flex items-center gap-2">
	<code
		class={cn(
			'flex-1 rounded-md bg-muted px-3 py-2 font-mono text-sm break-all',
			secret && REPLAY_BLOCK_CLASS
		)}>{value}</code
	>
	<CopyButton text={value} />
</div>
