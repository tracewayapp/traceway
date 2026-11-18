<script lang="ts">
	import type { SloStatus } from '$lib/state/slos.svelte';
	import StatusPill from '$lib/components/traceway/status-pill.svelte';

	interface Props {
		status: SloStatus;
		enabled?: boolean;
	}

	let { status, enabled = true }: Props = $props();
</script>

{#if !enabled}
	<StatusPill tone="neutral" dot label="Paused" />
{:else if status === 'ok'}
	<StatusPill tone="success" dot label="Meeting" />
{:else if status === 'breached'}
	<StatusPill tone="danger" dot label="Breached" />
{:else}
	<StatusPill tone="neutral" dot label="No data" />
{/if}
