<script lang="ts">
	import type { ComponentProps } from 'svelte';
	import * as Tooltip from '$lib/components/ui/tooltip';
	import SessionActionsTable from '$lib/components/issues/session-actions-table.svelte';

	let props: ComponentProps<typeof SessionActionsTable> = $props();
</script>

<Tooltip.Provider>
	<SessionActionsTable {...props} />
</Tooltip.Provider>
