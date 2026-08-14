<script lang="ts">
	import type { CheckType } from '$lib/state/monitors.svelte';

	interface Props {
		type: CheckType;
	}

	let { type }: Props = $props();

	const typeClasses: Record<CheckType, string> = {
		http: 'bg-blue-500/15 text-blue-600 dark:text-blue-400',
		tcp: 'bg-violet-500/15 text-violet-600 dark:text-violet-400',
		browser: 'bg-amber-500/15 text-amber-700 dark:text-amber-400'
	};
</script>

<span
	class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium tracking-wide uppercase {typeClasses[
		type
	] ?? 'bg-muted text-muted-foreground'}"
>
	{type}
</span>
