<script lang="ts">
	import * as Tooltip from '$lib/components/ui/tooltip';
	import SpanTracePage from '../routes/spans/[traceId]/+page.svelte';

	let { data }: { data: { traceId: string; at: string; spanId: string | null } } = $props();
</script>

<Tooltip.Provider>
	<SpanTracePage {data} />
</Tooltip.Provider>
