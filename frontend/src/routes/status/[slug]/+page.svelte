<script lang="ts">
	import PublicStatusPage from '$lib/components/traceway/public-status-page.svelte';

	let { data }: { data: { slug: string } } = $props();
</script>

<PublicStatusPage slug={data.slug} />
