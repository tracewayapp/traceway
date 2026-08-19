<script lang="ts">
	import { Button } from '$lib/components/ui/button';

	interface Props {
		message: string;
		onRetry?: () => void;
		retryLabel?: string;
	}

	let { message, onRetry, retryLabel = 'Retry' }: Props = $props();
</script>

<div class="flex flex-col items-center justify-center gap-3 rounded-md bg-muted py-20 text-center">
	<p class="text-sm text-destructive">{message}</p>
	{#if onRetry}
		<Button variant="outline" size="sm" onclick={onRetry}>{retryLabel}</Button>
	{/if}
</div>
