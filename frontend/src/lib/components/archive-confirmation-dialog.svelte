<script lang="ts">
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Button } from '$lib/components/ui/button';
	import { Checkbox } from '$lib/components/ui/checkbox';
	import { Archive } from '@lucide/svelte';
	import { api } from '$lib/api';
	import { projectsState } from '$lib/state/projects.svelte';

	interface Props {
		open: boolean;
		onOpenChange: (open: boolean) => void;
		count: number;
		hashes: string[];
		onConfirm: (resolvePages: boolean) => Promise<void>;
	}

	let { open, onOpenChange, count, hashes, onConfirm }: Props = $props();
	let loading = $state(false);
	let linkedPages = $state(0);
	let resolvePages = $state(true);

	// On-call pages opened for these issues (new-error/regression rules) can be
	// resolved together with the archive; the option only shows when some exist.
	$effect(() => {
		if (open) {
			resolvePages = true;
			linkedPages = 0;
			loadLinkedPages(hashes);
		}
	});

	async function loadLinkedPages(forHashes: string[]) {
		if (forHashes.length === 0) return;
		try {
			const res = await api.post(
				'/pages/for-issues',
				{ hashes: forHashes },
				{ projectId: projectsState.currentProjectId ?? undefined }
			);
			linkedPages = res.count ?? 0;
		} catch {
			linkedPages = 0;
		}
	}

	async function handleConfirm() {
		loading = true;
		try {
			await onConfirm(linkedPages > 0 && resolvePages);
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
				Are you sure you want to archive {count} issue{count > 1 ? 's' : ''}? Archived issues will
				be hidden from the main issues list.
			</AlertDialog.Description>
		</AlertDialog.Header>
		{#if linkedPages > 0}
			<label class="flex cursor-pointer items-center gap-2 text-sm">
				<Checkbox
					checked={resolvePages}
					onCheckedChange={(checked) => (resolvePages = checked === true)}
					disabled={loading}
					aria-label="Also resolve linked on-call pages"
				/>
				Also resolve {linkedPages} open on-call page{linkedPages > 1 ? 's' : ''} for
				{count > 1 ? 'these issues' : 'this issue'}
			</label>
		{/if}
		<AlertDialog.Footer>
			<Button variant="outline" onclick={handleCancel} disabled={loading}>Cancel</Button>
			<Button variant="destructiveOutline" onclick={handleConfirm} disabled={loading}>
				<Archive class="h-4 w-4" />
				{loading ? 'Archiving...' : 'Archive'}
			</Button>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
