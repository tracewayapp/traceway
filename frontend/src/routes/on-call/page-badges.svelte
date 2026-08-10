<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';

	interface Props {
		severity?: string;
		urgency?: string;
		status?: string;
		longUrgency?: boolean;
		fallback?: boolean;
	}

	let { severity, urgency, status, longUrgency = false, fallback = false }: Props = $props();
</script>

{#if severity === 'critical'}
	<Badge variant="destructive">Critical</Badge>
{:else if severity === 'warning'}
	<Badge class="bg-amber-500 text-white">Warning</Badge>
{:else if severity === 'info'}
	<Badge variant="secondary">Info</Badge>
{:else if fallback}
	<span class="text-muted-foreground">—</span>
{/if}

{#if urgency === 'high'}
	<Badge class="bg-rose-700 text-white">{longUrgency ? 'High urgency' : 'High'}</Badge>
{:else if urgency === 'low'}
	<Badge variant="secondary">{longUrgency ? 'Low urgency' : 'Low'}</Badge>
{/if}

{#if status === 'open'}
	<Badge variant="destructive">Open</Badge>
{:else if status === 'acknowledged'}
	<Badge class="bg-amber-500 text-white">Acknowledged</Badge>
{:else if status === 'resolved'}
	<Badge class="bg-green-600 text-white">Resolved</Badge>
{/if}
