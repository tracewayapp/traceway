<script lang="ts">
	import type { ComponentProps } from 'svelte';
	import * as Tooltip from '$lib/components/ui/tooltip';
	import EventCard from '$lib/components/issues/event-card.svelte';

	let props: ComponentProps<typeof EventCard> = $props();
</script>

<Tooltip.Provider>
	<EventCard {...props} />
</Tooltip.Provider>
