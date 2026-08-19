<script lang="ts">
	import type { Snippet } from 'svelte';
	import { Button, type ButtonVariant } from '$lib/components/ui/button';
	import { Plus } from '@lucide/svelte';

	interface Props {
		message: string;
		actionLabel?: string;
		actionVariant?: ButtonVariant;
		onAction?: () => void;
		icon?: Snippet;
	}

	let { message, actionLabel, actionVariant = 'success', onAction, icon }: Props = $props();
</script>

<div
	class="flex flex-col items-center justify-center rounded-md bg-muted py-20 text-center text-muted-foreground"
>
	{#if icon}
		<div class="mb-4 flex h-12 w-12 items-center justify-center rounded-full bg-background">
			{@render icon()}
		</div>
	{/if}
	<p class={actionLabel && onAction ? 'mb-4' : ''}>{message}</p>
	{#if actionLabel && onAction}
		<Button variant={actionVariant} onclick={onAction}>
			{#if actionVariant === 'success'}
				<Plus class="mr-2 h-4 w-4" />
			{/if}
			{actionLabel}
		</Button>
	{/if}
</div>
