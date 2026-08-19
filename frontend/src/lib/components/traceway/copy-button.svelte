<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Check, Copy } from '@lucide/svelte';
	import { cn } from '$lib/utils.js';

	interface Props {
		text: string | (() => string);
		label?: string;
		copiedLabel?: string;
		size?: 'sm' | 'default';
		bare?: boolean;
		class?: string;
		iconClass?: string;
		title?: string;
	}

	let {
		text,
		label,
		copiedLabel = 'Copied!',
		size = 'sm',
		bare = false,
		class: className,
		iconClass = 'h-4 w-4',
		title = 'Copy to clipboard'
	}: Props = $props();

	let copied = $state(false);
	let timer: ReturnType<typeof setTimeout> | undefined;

	async function copy() {
		const value = typeof text === 'function' ? text() : text;
		await navigator.clipboard.writeText(value);
		copied = true;
		clearTimeout(timer);
		timer = setTimeout(() => (copied = false), 2000);
	}
</script>

{#if bare}
	<button
		type="button"
		class={cn(
			'cursor-pointer rounded p-1 text-muted-foreground hover:bg-muted hover:text-foreground',
			className
		)}
		onclick={copy}
		{title}
	>
		{#if copied}
			<Check class={cn(iconClass, 'text-green-500')} />
		{:else}
			<Copy class={iconClass} />
		{/if}
		<span class="sr-only">{title}</span>
	</button>
{:else}
	<Button variant="outline" {size} class={className} onclick={copy} {title}>
		{#if copied}
			<Check class={cn(iconClass, 'text-green-500', label ? 'mr-2' : '')} />
			{#if label}{copiedLabel}{/if}
		{:else}
			<Copy class={cn(iconClass, label ? 'mr-2' : '')} />
			{#if label}{label}{/if}
		{/if}
	</Button>
{/if}
