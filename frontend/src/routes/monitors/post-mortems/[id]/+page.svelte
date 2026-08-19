<script lang="ts">
	import { getErrorMessage, getErrorStatus } from '$lib/utils/errors';
	import { untrack } from 'svelte';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { ErrorDisplay } from '$lib/components/ui/error-display';
	import { ErrorAlert } from '$lib/components/ui/error-alert';
	import PageHeader from '$lib/components/traceway/page-header.svelte';
	import ConfirmDeleteDialog from '$lib/components/traceway/confirm-delete-dialog.svelte';
	import TagsInput from '$lib/components/traceway/tags-input.svelte';
	import MarkdownEditor from '$lib/components/post-mortems/markdown-editor.svelte';
	import ActivitySheet from '$lib/components/post-mortems/activity-sheet.svelte';
	import { Check, History, Link2, Plus, Trash2, X } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';
	import { api } from '$lib/api';
	import { gotoHref } from '$lib/utils/navigation';
	import { createSmartBackHandler } from '$lib/utils/back-navigation';
	import { formatDateTime } from '$lib/utils/formatters';
	import { projectsState } from '$lib/state/projects.svelte';
	import {
		incidentDisplayTitle,
		type OrgIncident,
		type PostMortem
	} from '$lib/state/monitors.svelte';

	let { data } = $props();

	const postMortemId = $derived(data.id);
	const projectId = $derived(projectsState.currentProjectId);
	const organizationId = $derived(projectsState.currentProject?.organizationId ?? null);
	const canWrite = $derived(projectsState.canWriteCurrentProject);

	let loading = $state(true);
	let notFound = $state(false);
	let error = $state('');
	let formError = $state('');
	let saving = $state(false);
	let deleteOpen = $state(false);
	let deleting = $state(false);
	let activityOpen = $state(false);

	let title = $state('');
	let tags = $state<string[]>([]);
	let contentMd = $state('');
	let linkedIncidentId = $state('');

	let incidents = $state<OrgIncident[]>([]);

	let markdownEditor = $state<{ flushPendingChanges: () => void } | null>(null);

	let tagsDialogOpen = $state(false);
	let dialogTags = $state<string[]>([]);
	let incidentDialogOpen = $state(false);
	let incidentSearch = $state('');

	let loadSeq = 0;

	async function loadAll() {
		if (projectId === null || organizationId === null) return;
		const seq = ++loadSeq;
		loading = true;
		notFound = false;
		error = '';
		formError = '';
		try {
			const [postMortem, incidentsRes] = (await Promise.all([
				api.get(`/post-mortems/${postMortemId}`, { projectId: projectId ?? undefined }),
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
		void projectId;
		void organizationId;
		untrack(() => loadAll());
	});

	function incidentLabel(incident: OrgIncident): string {
		return `${incidentDisplayTitle(incident)} · ${formatDateTime(incident.startedAt, { format: 'short' })}`;
	}

	// Auto incidents from other projects are excluded: the post-mortem belongs
	// to this project, manual (status page) incidents carry no project.
	const selectableIncidents = $derived(
		incidents.filter(
			(incident) =>
				(!incident.postMortemId || String(incident.id) === linkedIncidentId) &&
				(!incident.projectId ||
					incident.projectId === projectId ||
					String(incident.id) === linkedIncidentId)
		)
	);

	const linkedIncidentLabel = $derived.by(() => {
		if (!linkedIncidentId) return 'Not linked';
		const incident = incidents.find((i) => String(i.id) === linkedIncidentId);
		return incident ? incidentLabel(incident) : `Incident #${linkedIncidentId}`;
	});

	const filteredDialogIncidents = $derived.by(() => {
		const query = incidentSearch.trim().toLowerCase();
		if (!query) return selectableIncidents;
		return selectableIncidents.filter((incident) =>
			incidentLabel(incident).toLowerCase().includes(query)
		);
	});

	function openIncidentDialog() {
		incidentSearch = '';
		incidentDialogOpen = true;
	}

	function linkIncident(incident: OrgIncident) {
		linkedIncidentId = String(incident.id);
		incidentDialogOpen = false;
	}

	function openTagsDialog() {
		dialogTags = [...tags];
		tagsDialogOpen = true;
	}

	function applyTags() {
		tags = dialogTags;
		tagsDialogOpen = false;
	}

	function removeTag(tag: string) {
		tags = tags.filter((t) => t !== tag);
	}

	async function save() {
		if (projectId === null) return;
		markdownEditor?.flushPendingChanges();
		saving = true;
		formError = '';
		try {
			const updated = (await api.put(
				`/post-mortems/${postMortemId}`,
				{
					title,
					contentMd,
					tags,
					incidentId: linkedIncidentId ? parseInt(linkedIncidentId, 10) : null
				},
				{ projectId: projectId ?? undefined }
			)) as PostMortem;
			title = updated.title;
			tags = updated.tags || [];
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
		if (projectId === null) return;
		deleting = true;
		try {
			await api.delete(`/post-mortems/${postMortemId}`, { projectId: projectId ?? undefined });
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
		description="This post-mortem does not exist or belongs to a different project."
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
				<Button variant="outline" size="sm" onclick={() => (activityOpen = true)}>
					<History class="mr-2 h-4 w-4" />
					Activity
				</Button>
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
		{/if}

		{#if canWrite || tags.length > 0 || linkedIncidentId}
			<div class="flex flex-wrap items-center gap-2">
				{#if linkedIncidentId}
					<span
						class="inline-flex items-center gap-1.5 rounded-full bg-muted px-3 py-0.5 text-xs font-medium"
					>
						<Link2 class="h-3 w-3 shrink-0 text-muted-foreground" />
						<span class="max-w-[320px] truncate">{linkedIncidentLabel}</span>
						{#if canWrite}
							<button
								type="button"
								aria-label="Unlink incident"
								class="ml-0.5 cursor-pointer text-muted-foreground hover:text-foreground"
								onclick={() => (linkedIncidentId = '')}
								disabled={saving}
							>
								<X class="h-3 w-3" />
							</button>
						{/if}
					</span>
				{:else if canWrite}
					<button
						type="button"
						class="inline-flex cursor-pointer items-center gap-1 rounded-full border border-dashed px-3 py-0.5 text-xs font-medium text-muted-foreground transition-colors hover:border-foreground/40 hover:text-foreground"
						onclick={openIncidentDialog}
						disabled={saving}
					>
						<Link2 class="h-3 w-3" />
						Link incident
					</button>
				{/if}
				{#each tags as tag (tag)}
					<span
						class="inline-flex items-center gap-1 rounded-full bg-muted px-2.5 py-0.5 text-xs font-medium"
					>
						{tag}
						{#if canWrite}
							<button
								type="button"
								aria-label={`Remove tag ${tag}`}
								class="ml-0.5 cursor-pointer text-muted-foreground hover:text-foreground"
								onclick={() => removeTag(tag)}
								disabled={saving}
							>
								<X class="h-3 w-3" />
							</button>
						{/if}
					</span>
				{/each}
				{#if canWrite}
					<button
						type="button"
						class="inline-flex cursor-pointer items-center gap-1 rounded-full border border-dashed px-3 py-0.5 text-xs font-medium text-muted-foreground transition-colors hover:border-foreground/40 hover:text-foreground"
						onclick={openTagsDialog}
						disabled={saving}
					>
						<Plus class="h-3 w-3" />
						Add tags
					</button>
				{/if}
			</div>
		{/if}

		{#key postMortemId}
			<MarkdownEditor
				bind:this={markdownEditor}
				bind:value={contentMd}
				readonly={!canWrite || saving}
			/>
		{/key}
	</div>

	<ConfirmDeleteDialog
		bind:open={deleteOpen}
		entity="Post-Mortem"
		description={`This permanently deletes "${title}". Linked incidents keep their public timeline.`}
		loading={deleting}
		onConfirm={deletePostMortem}
	/>

	<AlertDialog.Root bind:open={incidentDialogOpen}>
		<AlertDialog.Content class="sm:max-w-lg">
			<AlertDialog.Header>
				<AlertDialog.Title>Link Incident</AlertDialog.Title>
				<AlertDialog.Description>
					Link this post-mortem to the incident it covers. Incidents that already have a
					post-mortem are not listed.
				</AlertDialog.Description>
			</AlertDialog.Header>
			<div class="space-y-3">
				<Input bind:value={incidentSearch} placeholder="Search incidents..." />
				<div class="max-h-72 overflow-y-auto rounded-md border">
					{#each filteredDialogIncidents as incident (incident.id)}
						<button
							type="button"
							class="flex w-full cursor-pointer flex-col items-start gap-0.5 border-b px-3 py-2 text-left text-sm last:border-b-0 hover:bg-accent"
							onclick={() => linkIncident(incident)}
						>
							<span class="font-medium">{incidentDisplayTitle(incident)}</span>
							<span class="text-xs text-muted-foreground">
								{formatDateTime(incident.startedAt, { format: 'short' })}
							</span>
						</button>
					{:else}
						<div class="py-8 text-center text-sm text-muted-foreground">No incidents found</div>
					{/each}
				</div>
			</div>
			<AlertDialog.Footer>
				<AlertDialog.Cancel>Cancel</AlertDialog.Cancel>
			</AlertDialog.Footer>
		</AlertDialog.Content>
	</AlertDialog.Root>

	<AlertDialog.Root bind:open={tagsDialogOpen}>
		<AlertDialog.Content class="sm:max-w-md">
			<AlertDialog.Header>
				<AlertDialog.Title>Add Tags</AlertDialog.Title>
				<AlertDialog.Description>
					Type a tag and press Enter; add as many as you need.
				</AlertDialog.Description>
			</AlertDialog.Header>
			<TagsInput bind:tags={dialogTags} />
			<AlertDialog.Footer>
				<AlertDialog.Cancel>Cancel</AlertDialog.Cancel>
				<Button onclick={applyTags}>
					<Check class="mr-2 h-4 w-4" />
					Apply Tags
				</Button>
			</AlertDialog.Footer>
		</AlertDialog.Content>
	</AlertDialog.Root>

	<ActivitySheet bind:open={activityOpen} {postMortemId} />
{/if}
