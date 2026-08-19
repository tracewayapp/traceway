<script lang="ts">
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { ErrorAlert } from '$lib/components/ui/error-alert';
	import { Plus } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';
	import { api } from '$lib/api';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { projectsState } from '$lib/state/projects.svelte';

	interface Props {
		open: boolean;
		incidentId?: number | null;
		incidentLabel?: string;
	}

	let { open = $bindable(), incidentId = null, incidentLabel = '' }: Props = $props();

	const organizationId = $derived(projectsState.currentProject?.organizationId ?? null);

	let title = $state('');
	let saving = $state(false);
	let dialogError = $state('');

	$effect(() => {
		if (open) {
			title = '';
			dialogError = '';
		}
	});

	async function create() {
		if (organizationId === null) return;
		saving = true;
		dialogError = '';
		try {
			const created = await api.post(
				`/organizations/${organizationId}/post-mortems`,
				{ title, contentMd: '', tags: [], incidentId },
				{ skipProjectId: true }
			);
			toast.success('Successfully created the Post-Mortem', { position: 'top-center' });
			open = false;
			goto(resolve(`/monitors/post-mortems/${created.id}`));
		} catch (e: any) {
			if (e?.status !== 403) {
				dialogError = e instanceof Error ? e.message : 'Failed to create the post-mortem';
			}
		} finally {
			saving = false;
		}
	}
</script>

<AlertDialog.Root bind:open>
	<AlertDialog.Content class="sm:max-w-md" interactOutsideBehavior={saving ? 'ignore' : 'close'}>
		<AlertDialog.Header>
			<AlertDialog.Title>New Post-Mortem</AlertDialog.Title>
			<AlertDialog.Description>
				{#if incidentId}
					Linked to: {incidentLabel || `incident #${incidentId}`}.
				{/if}
				The post-mortem is created right away; you write the document, add tags, and manage the incident
				link on the next screen.
			</AlertDialog.Description>
		</AlertDialog.Header>

		<form
			onsubmit={(e) => {
				e.preventDefault();
				create();
			}}
			class="space-y-4"
		>
			<ErrorAlert error={dialogError} />
			<div class="space-y-2">
				<Label for="new-post-mortem-title">Title</Label>
				<Input
					id="new-post-mortem-title"
					bind:value={title}
					placeholder="What went wrong, in one line"
					disabled={saving}
				/>
			</div>
		</form>

		<AlertDialog.Footer>
			<AlertDialog.Cancel disabled={saving}>Cancel</AlertDialog.Cancel>
			<Button variant="success" onclick={create} disabled={saving}>
				<Plus class="mr-2 h-4 w-4" />
				{saving ? 'Creating...' : 'New Post-Mortem'}
			</Button>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
