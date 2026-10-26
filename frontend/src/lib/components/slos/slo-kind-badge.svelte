<script lang="ts">
	import { SLI_KIND_LABELS, type SliKind } from '$lib/state/slos.svelte';

	interface Props {
		kind: SliKind;
	}

	let { kind }: Props = $props();

	const kindClasses: Record<SliKind, string> = {
		endpoint: 'bg-blue-500/15 text-blue-600 dark:text-blue-400',
		span: 'bg-violet-500/15 text-violet-600 dark:text-violet-400',
		task: 'bg-cyan-500/15 text-cyan-700 dark:text-cyan-400',
		monitor: 'bg-amber-500/15 text-amber-700 dark:text-amber-400',
		metric: 'bg-emerald-500/15 text-emerald-700 dark:text-emerald-400',
		session: 'bg-rose-500/15 text-rose-600 dark:text-rose-400'
	};
</script>

<span
	class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium tracking-wide uppercase {kindClasses[
		kind
	] ?? 'bg-muted text-muted-foreground'}"
>
	{SLI_KIND_LABELS[kind] ?? kind}
</span>
