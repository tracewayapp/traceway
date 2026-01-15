<script lang="ts">
	import * as AlertDialog from "$lib/components/ui/alert-dialog";
	import { Button } from "$lib/components/ui/button";
	import { Archive } from "lucide-svelte";

	interface Props {
		open: boolean;
		onOpenChange: (open: boolean) => void;
		count: number;
		onConfirm: () => Promise<void>;
	}

	let { open, onOpenChange, count, onConfirm }: Props = $props();
	let loading = $state(false);

	async function handleConfirm() {
		loading = true;
		try {
			await onConfirm();
			onOpenChange(false);
		} catch (e) {
			// Error handling done in parent, just reset loading
		} finally {
			loading = false;
		}
	}

	function handleCancel() {
		if (!loading) {
			onOpenChange(false);
		}
	}
</script>

<AlertDialog.Root {open} {onOpenChange}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>Archive Issue{count > 1 ? 's' : ''}</AlertDialog.Title>
			<AlertDialog.Description>
				Are you sure you want to archive {count} issue{count > 1 ? 's' : ''}?
				Archived issues will be hidden from the main issues list.
			</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<Button variant="outline" onclick={handleCancel} disabled={loading}>
				Cancel
			</Button>
			<Button variant="destructiveOutline" onclick={handleConfirm} disabled={loading}>
				<Archive class="h-4 w-4" /> {loading ? 'Archiving...' : 'Archive'}
			</Button>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
