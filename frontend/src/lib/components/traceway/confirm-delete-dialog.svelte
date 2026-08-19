<script lang="ts">
	import type { Snippet } from 'svelte';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Button } from '$lib/components/ui/button';
	import { ErrorAlert } from '$lib/components/ui/error-alert';
	import { Trash2 } from '@lucide/svelte';

	interface Props {
		open?: boolean;
		entity: string;
		verb?: 'Delete' | 'Revoke' | 'Remove';
		title?: string;
		description?: string;
		error?: string | null;
		loading?: boolean;
		onConfirm: () => void;
		children?: Snippet;
	}

	let {
		open = $bindable(false),
		entity,
		verb = 'Delete',
		title,
		description,
		error = null,
		loading = false,
		onConfirm,
		children
	}: Props = $props();
</script>

<AlertDialog.Root bind:open>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>{title ?? `${verb} ${entity}`}</AlertDialog.Title>
			{#if description}
				<AlertDialog.Description>{description}</AlertDialog.Description>
			{/if}
		</AlertDialog.Header>
		{#if children}{@render children()}{/if}
		<ErrorAlert {error} />
		<AlertDialog.Footer>
			<AlertDialog.Cancel disabled={loading}>Cancel</AlertDialog.Cancel>
			<Button variant="destructive" onclick={onConfirm} disabled={loading}>
				<Trash2 class="mr-2 h-4 w-4" />
				{verb}
				{entity}
			</Button>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
