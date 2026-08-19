<script lang="ts">
	import type { Snippet } from 'svelte';
	import { cn } from '$lib/utils.js';

	export type PillTone = 'success' | 'danger' | 'warning' | 'info' | 'neutral' | 'violet';

	interface Props {
		tone: PillTone;
		label?: string;
		dot?: boolean;
		class?: string;
		children?: Snippet;
	}

	let { tone, label, dot = false, class: className, children }: Props = $props();

	const tones: Record<PillTone, string> = {
		success: 'bg-green-500/15 text-green-600 dark:text-green-400',
		danger: 'bg-red-500/15 text-red-600 dark:text-red-400',
		warning: 'bg-amber-500/15 text-amber-700 dark:text-amber-400',
		info: 'bg-blue-500/15 text-blue-600 dark:text-blue-400',
		neutral: 'bg-muted text-muted-foreground',
		violet: 'bg-violet-500/15 text-violet-600 dark:text-violet-400'
	};

	const dots: Record<PillTone, string> = {
		success: 'bg-green-500',
		danger: 'bg-red-500',
		warning: 'bg-amber-500',
		info: 'bg-blue-500',
		neutral: 'bg-muted-foreground/60',
		violet: 'bg-violet-500'
	};
</script>

<span
	class={cn(
		'inline-flex shrink-0 items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium whitespace-nowrap',
		tones[tone],
		className
	)}
>
	{#if dot}<span class={cn('h-1.5 w-1.5 rounded-full', dots[tone])}></span>{/if}
	{#if children}{@render children()}{:else}{label}{/if}
</span>
