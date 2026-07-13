<script lang="ts">
	let { endpoint }: { endpoint: string } = $props();

	const METHOD_COLORS: Record<string, string> = {
		GET: 'text-sky-600 dark:text-sky-400',
		POST: 'text-emerald-600 dark:text-emerald-400',
		PUT: 'text-amber-600 dark:text-amber-400',
		PATCH: 'text-orange-600 dark:text-orange-400',
		DELETE: 'text-rose-600 dark:text-rose-400',
		HEAD: 'text-violet-600 dark:text-violet-400',
		OPTIONS: 'text-violet-600 dark:text-violet-400'
	};

	const parts = $derived.by(() => {
		const spaceIndex = endpoint.indexOf(' ');
		if (spaceIndex > 0) {
			const method = endpoint.slice(0, spaceIndex);
			if (METHOD_COLORS[method]) {
				return { method, path: endpoint.slice(spaceIndex + 1), color: METHOD_COLORS[method] };
			}
		}
		return { method: '', path: endpoint, color: '' };
	});
</script>

{#if parts.method}
	<span class="font-semibold {parts.color}">{parts.method}</span>
	<span class="text-foreground">{parts.path}</span>
{:else}
	<span class="text-foreground">{parts.path}</span>
{/if}
