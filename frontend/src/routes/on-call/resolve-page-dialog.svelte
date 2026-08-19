<script lang="ts">
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Button } from '$lib/components/ui/button';
	import { Checkbox } from '$lib/components/ui/checkbox';
	import { CheckCheck } from '@lucide/svelte';
	import { projectsState } from '$lib/state/projects.svelte';
	import { type OncallPage } from '$lib/state/oncall.svelte';
	import { runBulkPageAction } from './page-actions';

	interface Props {
		open: boolean;
		pages: OncallPage[];
		onResolved: () => void;
	}

	let { open = $bindable(), pages, onResolved }: Props = $props();
	let loading = $state(false);
	let archiveIssues = $state(true);

	const linkedIssueCount = $derived(new Set(pages.map((p) => p.issueHash).filter(Boolean)).size);
	// Archiving is a write on the issues while resolving is not, so the option
	// is only offered when pages are issue-linked and the user can write.
	const canArchiveIssues = $derived(linkedIssueCount > 0 && projectsState.canWriteCurrentProject);

	$effect(() => {
		if (open) {
			archiveIssues = true;
		}
	});

	async function handleConfirm() {
		if (pages.length === 0) return;
		loading = true;
		try {
			const ok = await runBulkPageAction(
				pages.map((p) => p.id),
				'resolve',
				{ archiveIssues: canArchiveIssues && archiveIssues }
			);
			if (ok) {
				open = false;
				onResolved();
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
			<AlertDialog.Title>Resolve Page{pages.length > 1 ? 's' : ''}</AlertDialog.Title>
			<AlertDialog.Description>
				{#if pages.length > 1}
					Are you sure you want to resolve {pages.length} pages?
				{:else if pages[0]?.subject}
					Are you sure you want to resolve the page "{pages[0].subject}"?
				{:else}
					Are you sure you want to resolve this page?
				{/if}
				Resolving stops all escalations for {pages.length > 1 ? 'them' : 'it'}.
			</AlertDialog.Description>
		</AlertDialog.Header>
		{#if canArchiveIssues}
			<label class="flex cursor-pointer items-center gap-2 text-sm">
				<Checkbox
					checked={archiveIssues}
					onCheckedChange={(checked) => (archiveIssues = checked === true)}
					disabled={loading}
					aria-label="Also archive the linked issues"
				/>
				{linkedIssueCount > 1
					? `Also archive the ${linkedIssueCount} linked issues`
					: 'Also archive the linked issue'}
			</label>
		{/if}
		<AlertDialog.Footer>
			<Button variant="outline" onclick={handleCancel} disabled={loading}>Cancel</Button>
			<Button onclick={handleConfirm} disabled={loading}>
				<CheckCheck class="h-4 w-4" />
				{loading ? 'Resolving...' : `Resolve Page${pages.length > 1 ? 's' : ''}`}
			</Button>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
