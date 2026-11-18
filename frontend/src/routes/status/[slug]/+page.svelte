<script lang="ts">
	import StatusPageView from '$lib/components/traceway/status-page-view.svelte';

	let { data }: { data: { slug: string } } = $props();
</script>

<StatusPageView slug={data.slug} />
