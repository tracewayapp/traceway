<script lang="ts">
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Button } from '$lib/components/ui/button';
	import { Check } from '@lucide/svelte';
	import { type OncallPage } from '$lib/state/oncall.svelte';
	import { runBulkPageAction } from './page-actions';

	interface Props {
		open: boolean;
		pages: OncallPage[];
		onAcknowledged: () => void;
	}

	let { open = $bindable(), pages, onAcknowledged }: Props = $props();
	let loading = $state(false);

	async function handleConfirm() {
		if (pages.length === 0) return;
		loading = true;
		try {
			const ok = await runBulkPageAction(
				pages.map((p) => p.id),
				'acknowledge'
			);
			if (ok) {
				open = false;
				onAcknowledged();
			}
		} finally {
			loading = false;
		}
	}

	function handleCancel() {
		if (!loading) {
			open = false;
		}
	}
</script>

<AlertDialog.Root bind:open>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>Acknowledge Page{pages.length > 1 ? 's' : ''}</AlertDialog.Title>
			<AlertDialog.Description>
				Are you sure you want to acknowledge {pages.length === 1
					? 'this page'
					: `${pages.length} pages`}? Acknowledging pauses escalation until the
				{pages.length > 1 ? 'pages are' : 'page is'} resolved.
			</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<Button variant="outline" onclick={handleCancel} disabled={loading}>Cancel</Button>
			<Button onclick={handleConfirm} disabled={loading}>
				<Check class="h-4 w-4" />
				{loading ? 'Acknowledging...' : `Acknowledge Page${pages.length > 1 ? 's' : ''}`}
			</Button>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
