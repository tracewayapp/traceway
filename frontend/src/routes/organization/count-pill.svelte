<script lang="ts">
	interface Props {
		count: number;
		label?: string;
		tone?: 'default' | 'danger';
	}

	let { count, label = '', tone = 'default' }: Props = $props();
</script>

<span
	class="mt-1 rounded-full px-2 py-0.5 font-mono text-xs tabular-nums {tone === 'danger'
		? 'bg-rose-500/10 font-semibold text-rose-600 dark:text-rose-400'
		: 'bg-muted text-muted-foreground'}"
>
	{count.toLocaleString()}{label ? ` ${label}` : ''}
</span>
