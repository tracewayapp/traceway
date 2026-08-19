<script lang="ts">
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Checkbox } from '$lib/components/ui/checkbox';
	import { ErrorAlert } from '$lib/components/ui/error-alert';
	import { Plus } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';
	import { api } from '$lib/api';
	import { toLocalInputValue } from '$lib/utils/formatters';
	import { projectsState } from '$lib/state/projects.svelte';

	interface Props {
		open: boolean;
		statusPage: { id: number; name: string } | null;
		onSaved: () => void;
	}

	let { open = $bindable(), statusPage, onSaved }: Props = $props();

	const organizationId = $derived(projectsState.currentProject?.organizationId ?? null);

	let title = $state('');
	let message = $state('');
	let startedAt = $state('');
	let alreadyResolved = $state(false);
	let resolvedAt = $state('');
	let saving = $state(false);
	let dialogError = $state('');

	$effect(() => {
		if (open) {
			title = '';
			message = '';
			startedAt = toLocalInputValue(new Date());
			alreadyResolved = false;
			resolvedAt = toLocalInputValue(new Date());
			dialogError = '';
		}
	});

	async function save() {
		if (!statusPage || organizationId === null) return;
		saving = true;
		dialogError = '';
		try {
			await api.post(
				`/organizations/${organizationId}/status-pages/${statusPage.id}/incidents`,
				{
					title: title.trim(),
					message: message.trim(),
					startedAt: startedAt ? new Date(startedAt).toISOString() : '',
					resolvedAt: alreadyResolved && resolvedAt ? new Date(resolvedAt).toISOString() : ''
				},
				{ skipProjectId: true }
			);
			toast.success('Successfully created the Incident', { position: 'top-center' });
			open = false;
			onSaved();
		} catch (e) {
			dialogError = e instanceof Error ? e.message : 'Failed to record the incident';
		} finally {
			saving = false;
		}
	}
</script>

<AlertDialog.Root bind:open>
	<AlertDialog.Content class="sm:max-w-lg" interactOutsideBehavior={saving ? 'ignore' : 'close'}>
		<AlertDialog.Header>
			<AlertDialog.Title>Record Incident</AlertDialog.Title>
			<AlertDialog.Description>
				Publishes an incident on "{statusPage?.name}" for an outage the monitors did not catch. It
				does not change any monitor's uptime numbers.
			</AlertDialog.Description>
		</AlertDialog.Header>

		<form
			onsubmit={(e) => {
				e.preventDefault();
				save();
			}}
			class="space-y-4"
		>
			<ErrorAlert error={dialogError} />
			<div class="space-y-2">
				<Label for="incident-title">Title</Label>
				<Input
					id="incident-title"
					bind:value={title}
					placeholder="Degraded performance for the API"
					disabled={saving}
				/>
			</div>
			<div class="space-y-2">
				<Label for="incident-message">First update (optional)</Label>
				<textarea
					id="incident-message"
					bind:value={message}
					rows="3"
					placeholder="We are investigating reports of degraded performance."
					disabled={saving}
					class="w-full resize-y rounded-md border border-input bg-transparent p-3 text-sm shadow-xs focus-visible:ring-1 focus-visible:ring-ring focus-visible:outline-none"
				></textarea>
			</div>
			<div class="grid gap-4 sm:grid-cols-2">
				<div class="space-y-2">
					<Label for="incident-started">Started at</Label>
					<Input
						id="incident-started"
						type="datetime-local"
						bind:value={startedAt}
						disabled={saving}
					/>
				</div>
				<div class="space-y-2">
					<Label for="incident-resolved" class={alreadyResolved ? '' : 'text-muted-foreground'}>
						Resolved at
					</Label>
					<Input
						id="incident-resolved"
						type="datetime-local"
						bind:value={resolvedAt}
						disabled={saving || !alreadyResolved}
					/>
				</div>
			</div>
			<label class="flex cursor-pointer items-center gap-2 text-sm">
				<Checkbox
					checked={alreadyResolved}
					onCheckedChange={(checked) => (alreadyResolved = checked === true)}
					disabled={saving}
				/>
				Already resolved (recording it after the fact)
			</label>
		</form>

		<AlertDialog.Footer>
			<AlertDialog.Cancel disabled={saving}>Cancel</AlertDialog.Cancel>
			<Button variant="success" onclick={save} disabled={saving}>
				<Plus class="mr-2 h-4 w-4" />
				{saving ? 'Creating...' : 'New Incident'}
			</Button>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
