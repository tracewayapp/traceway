<script lang="ts">
	import { getErrorMessage, getErrorStatus } from '$lib/utils/errors';
	import { untrack } from 'svelte';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Badge } from '$lib/components/ui/badge';
	import * as Select from '$lib/components/ui/select';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { ErrorDisplay } from '$lib/components/ui/error-display';
	import { ErrorAlert } from '$lib/components/ui/error-alert';
	import PageHeader from '$lib/components/traceway/page-header.svelte';
	import ConfirmDeleteDialog from '$lib/components/traceway/confirm-delete-dialog.svelte';
	import TagsInput from '$lib/components/traceway/tags-input.svelte';
	import MarkdownEditor from '$lib/components/post-mortems/markdown-editor.svelte';
	import { Check, Trash2 } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';
	import { api } from '$lib/api';
	import { gotoHref } from '$lib/utils/navigation';
	import { createSmartBackHandler } from '$lib/utils/back-navigation';
	import { formatDateTime } from '$lib/utils/formatters';
	import { projectsState } from '$lib/state/projects.svelte';
	import { authState } from '$lib/state/auth.svelte';
	import {
		incidentDisplayTitle,
		type OrgIncident,
		type PostMortem
	} from '$lib/state/monitors.svelte';

	let { data } = $props();

	const postMortemId = $derived(data.id);
	const organizationId = $derived(projectsState.currentProject?.organizationId ?? null);
	const canWrite = $derived(
		organizationId !== null && authState.canWriteOrganization(organizationId)
	);

	let loading = $state(true);
	let notFound = $state(false);
	let error = $state('');
	let formError = $state('');
	let saving = $state(false);
	let deleteOpen = $state(false);
	let deleting = $state(false);

	let title = $state('');
	let tags = $state<string[]>([]);
	let contentMd = $state('');
	let linkedIncidentId = $state('');

	let incidents = $state<OrgIncident[]>([]);

	let loadSeq = 0;

	async function loadAll() {
		if (organizationId === null) return;
		const seq = ++loadSeq;
		loading = true;
		notFound = false;
		error = '';
		formError = '';
		try {
			const [postMortem, incidentsRes] = (await Promise.all([
				api.get(`/organizations/${organizationId}/post-mortems/${postMortemId}`, {
					skipProjectId: true
				}),
				api.get(`/organizations/${organizationId}/incidents`, { skipProjectId: true })
			])) as [PostMortem, { incidents?: OrgIncident[] }];
			if (seq !== loadSeq) return;
			title = postMortem.title;
			tags = postMortem.tags || [];
			contentMd = postMortem.contentMd;
			linkedIncidentId = postMortem.incidentId ? String(postMortem.incidentId) : '';
			incidents = incidentsRes.incidents || [];
			if (linkedIncidentId && !incidents.some((i) => String(i.id) === linkedIncidentId)) {
				try {
					const linked = await api.get(
						`/organizations/${organizationId}/incidents/${linkedIncidentId}/updates`,
						{ skipProjectId: true }
					);
					if (seq !== loadSeq) return;
					if (linked.incident) incidents = [linked.incident, ...incidents];
				} catch {
					linkedIncidentId = '';
				}
			}
		} catch (e) {
			if (seq !== loadSeq) return;
			if (getErrorStatus(e) === 404) {
				notFound = true;
			} else {
				error = e instanceof Error ? getErrorMessage(e) : 'Failed to load the post-mortem';
			}
		} finally {
			if (seq === loadSeq) loading = false;
		}
	}

	$effect(() => {
		void postMortemId;
		void organizationId;
		untrack(() => loadAll());
	});

	function incidentLabel(incident: OrgIncident): string {
		return `${incidentDisplayTitle(incident)} · ${formatDateTime(incident.startedAt, { format: 'short' })}`;
	}

	const selectableIncidents = $derived(
		incidents.filter(
			(incident) => !incident.postMortemId || String(incident.id) === linkedIncidentId
		)
	);

	const linkedIncidentLabel = $derived.by(() => {
		if (!linkedIncidentId) return 'Not linked';
		const incident = incidents.find((i) => String(i.id) === linkedIncidentId);
		return incident ? incidentLabel(incident) : `Incident #${linkedIncidentId}`;
	});

	async function save() {
		if (organizationId === null) return;
		saving = true;
		formError = '';
		try {
			await api.put(
				`/organizations/${organizationId}/post-mortems/${postMortemId}`,
				{
					title,
					contentMd,
					tags,
					incidentId: linkedIncidentId ? parseInt(linkedIncidentId, 10) : null
				},
				{ skipProjectId: true }
			);
			toast.success('Successfully updated the Post-Mortem', { position: 'top-center' });
		} catch (e) {
			if (getErrorStatus(e) !== 403) {
				formError = e instanceof Error ? getErrorMessage(e) : 'Failed to save the post-mortem';
			}
		} finally {
			saving = false;
		}
	}

	async function deletePostMortem() {
		if (organizationId === null) return;
		deleting = true;
		try {
			await api.delete(`/organizations/${organizationId}/post-mortems/${postMortemId}`, {
				skipProjectId: true
			});
			toast.success('Successfully deleted the Post-Mortem');
			gotoHref('/monitors?tab=post-mortems');
		} catch (e) {
			if (getErrorStatus(e) !== 403) {
				toast.error(getErrorMessage(e) || 'Failed to delete the post-mortem');
			}
		}
		deleting = false;
		deleteOpen = false;
	}

	const backHandler = createSmartBackHandler({ fallbackPath: '/monitors?tab=post-mortems' });
</script>

{#if loading}
	<div class="flex h-64 items-center justify-center">
		<LoadingCircle size="xlg" />
	</div>
{:else if notFound}
	<ErrorDisplay
		status={404}
		title="Post-mortem not found"
		description="This post-mortem does not exist or belongs to a different organization."
		onBack={backHandler}
		backLabel="Back to Post-Mortems"
	/>
{:else if error}
	<ErrorDisplay
		status={400}
		title="Error"
		description={error}
		onRetry={() => loadAll()}
		onBack={backHandler}
		backLabel="Back to Post-Mortems"
	/>
{:else}
	<div class="space-y-4">
		<PageHeader title={canWrite ? 'Post-Mortem' : title || 'Post-Mortem'} onBack={backHandler}>
			{#snippet actions()}
				{#if canWrite}
					<Button variant="destructiveOutline" size="sm" onclick={() => (deleteOpen = true)}>
						<Trash2 class="mr-2 h-4 w-4" />
						Delete
					</Button>
					<Button size="sm" onclick={save} disabled={saving}>
						<Check class="mr-2 h-4 w-4" />
						{saving ? 'Updating...' : 'Update Post-Mortem'}
					</Button>
				{/if}
			{/snippet}
		</PageHeader>

		<ErrorAlert error={formError} />

		{#if canWrite}
			<div class="space-y-2">
				<Label for="post-mortem-title">Title</Label>
				<Input
					id="post-mortem-title"
					bind:value={title}
					placeholder="What went wrong, in one line"
					disabled={saving}
				/>
			</div>
			<div class="grid gap-4 sm:grid-cols-2">
				<div class="min-w-0 space-y-2">
					<Label>Linked incident</Label>
					<Select.Root type="single" bind:value={linkedIncidentId} disabled={saving}>
						<Select.Trigger class="w-full min-w-0">{linkedIncidentLabel}</Select.Trigger>
						<Select.Content>
							<Select.Item value="">Not linked</Select.Item>
							{#each selectableIncidents as incident (incident.id)}
								<Select.Item value={String(incident.id)}>{incidentLabel(incident)}</Select.Item>
							{/each}
						</Select.Content>
					</Select.Root>
				</div>
				<div class="min-w-0 space-y-2">
					<Label>Tags</Label>
					<TagsInput bind:tags disabled={saving} />
				</div>
			</div>
		{:else}
			<div class="flex flex-wrap items-center gap-3">
				{#if tags.length > 0}
					<div class="flex flex-wrap gap-1">
						{#each tags as tag (tag)}
							<Badge variant="secondary">{tag}</Badge>
						{/each}
					</div>
				{/if}
				{#if linkedIncidentId}
					<span class="text-sm text-muted-foreground">Incident: {linkedIncidentLabel}</span>
				{/if}
			</div>
		{/if}

		<MarkdownEditor bind:value={contentMd} readonly={!canWrite || saving} />
	</div>

	<ConfirmDeleteDialog
		bind:open={deleteOpen}
		entity="Post-Mortem"
		description={`This permanently deletes "${title}". Linked incidents keep their public timeline.`}
		loading={deleting}
		onConfirm={deletePostMortem}
	/>
{/if}
