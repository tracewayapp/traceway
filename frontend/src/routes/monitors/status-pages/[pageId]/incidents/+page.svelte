<script lang="ts">
	import { untrack } from 'svelte';
	import * as Select from '$lib/components/ui/select';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { PaginationFooter } from '$lib/components/ui/pagination-footer';
	import { ErrorDisplay } from '$lib/components/ui/error-display';
	import PageHeader from '$lib/components/traceway/page-header.svelte';
	import StatusPill from '$lib/components/traceway/status-pill.svelte';
	import ConfirmDeleteDialog from '$lib/components/traceway/confirm-delete-dialog.svelte';
	import RecordIncidentDialog from '../../../record-incident-dialog.svelte';
	import NewPostMortemDialog from '../../../new-post-mortem-dialog.svelte';
	import {
		Check,
		ChevronDown,
		ChevronRight,
		FileText,
		Megaphone,
		Pencil,
		Send,
		Trash2
	} from '@lucide/svelte';
	import { toast } from 'svelte-sonner';
	import { api } from '$lib/api';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { createSmartBackHandler } from '$lib/utils/back-navigation';
	import { formatDateTime } from '$lib/utils/formatters';
	import { projectsState } from '$lib/state/projects.svelte';
	import {
		incidentDisplayTitle,
		type IncidentUpdate,
		type IncidentUpdateStatus,
		type OrgIncident
	} from '$lib/state/monitors.svelte';

	let { data } = $props();

	const organizationId = $derived(projectsState.currentProject?.organizationId ?? null);
	const canManage = $derived(projectsState.canManageCurrentProject);

	let statusPageName = $state('');
	let incidents = $state<OrgIncident[]>([]);
	let loading = $state(true);
	let notFound = $state(false);
	let error = $state('');
	let currentPage = $state(1);
	let pageSize = $state(25);
	let total = $state(0);
	let totalPages = $state(0);

	let expandedId = $state<number | null>(null);
	let updates = $state<IncidentUpdate[]>([]);
	let updatesLoading = $state(false);

	let updateStatus = $state<IncidentUpdateStatus>('investigating');
	let updateMessage = $state('');
	let posting = $state(false);

	let editingTitle = $state(false);
	let titleDraft = $state('');
	let savingTitle = $state(false);

	let deleteTarget = $state<OrgIncident | null>(null);
	let deleting = $state(false);
	let recordOpen = $state(false);

	const statusOptions: { value: IncidentUpdateStatus; label: string }[] = [
		{ value: 'investigating', label: 'Investigating' },
		{ value: 'identified', label: 'Identified' },
		{ value: 'monitoring', label: 'Monitoring' },
		{ value: 'update', label: 'Update' },
		{ value: 'resolved', label: 'Resolved' }
	];

	const statusLabel = $derived(
		statusOptions.find((o) => o.value === updateStatus)?.label || 'Status'
	);

	let loadSeq = 0;

	async function loadIncidents() {
		if (organizationId === null) return;
		const seq = ++loadSeq;
		loading = true;
		error = '';
		try {
			const params = new URLSearchParams({
				page: String(currentPage),
				pageSize: String(pageSize)
			});
			const res = await api.get(
				`/organizations/${organizationId}/status-pages/${data.pageId}/incidents?${params}`,
				{ skipProjectId: true }
			);
			if (seq !== loadSeq) return;
			statusPageName = res.statusPage?.name || '';
			incidents = res.data || [];
			total = res.pagination?.total || 0;
			totalPages = res.pagination?.totalPages || 0;
		} catch (e: any) {
			if (seq !== loadSeq) return;
			if (e?.status === 404) {
				notFound = true;
			} else {
				error = e instanceof Error ? e.message : 'Failed to load incidents';
			}
		} finally {
			if (seq === loadSeq) loading = false;
		}
	}

	$effect(() => {
		void organizationId;
		void data.pageId;
		untrack(() => {
			expandedId = null;
			loadIncidents();
		});
	});

	async function loadUpdates(incidentId: number) {
		updatesLoading = true;
		updates = [];
		try {
			const res = await api.get(
				`/organizations/${organizationId}/incidents/${incidentId}/updates`,
				{
					skipProjectId: true
				}
			);
			updates = res.updates || [];
		} catch {
			updates = [];
		} finally {
			updatesLoading = false;
		}
	}

	function toggleExpand(incident: OrgIncident) {
		editingTitle = false;
		if (expandedId === incident.id) {
			expandedId = null;
			return;
		}
		expandedId = incident.id;
		updateStatus = incident.resolvedAt ? 'update' : 'investigating';
		updateMessage = '';
		loadUpdates(incident.id);
	}

	async function postUpdate(incident: OrgIncident, status: IncidentUpdateStatus = updateStatus) {
		posting = true;
		try {
			await api.post(
				`/organizations/${organizationId}/incidents/${incident.id}/updates`,
				{ status, message: updateMessage.trim() },
				{ skipProjectId: true }
			);
			updateMessage = '';
			await Promise.all([loadUpdates(incident.id), loadIncidents()]);
		} catch (e: any) {
			if (e?.status !== 403) {
				toast.error(e?.message || 'Failed to post the update');
			}
		} finally {
			posting = false;
		}
	}

	let deleteUpdateTarget = $state<{ incident: OrgIncident; update: IncidentUpdate } | null>(null);
	let deletingUpdate = $state(false);

	async function deleteUpdate() {
		if (!deleteUpdateTarget) return;
		const { incident, update } = deleteUpdateTarget;
		deletingUpdate = true;
		try {
			await api.delete(
				`/organizations/${organizationId}/incidents/${incident.id}/updates/${update.id}`,
				{ skipProjectId: true }
			);
			deleteUpdateTarget = null;
			await loadUpdates(incident.id);
		} catch (e: any) {
			if (e?.status !== 403) {
				toast.error(e?.message || 'Failed to delete the update');
			}
		} finally {
			deletingUpdate = false;
		}
	}

	function startTitleEdit(incident: OrgIncident) {
		editingTitle = true;
		titleDraft = incident.title || '';
	}

	async function saveTitle(incident: OrgIncident) {
		savingTitle = true;
		try {
			await api.put(
				`/organizations/${organizationId}/incidents/${incident.id}`,
				{ title: titleDraft.trim() },
				{ skipProjectId: true }
			);
			editingTitle = false;
			await loadIncidents();
		} catch (e: any) {
			if (e?.status !== 403) {
				toast.error(e?.message || 'Failed to update the title');
			}
		} finally {
			savingTitle = false;
		}
	}

	async function deleteIncident() {
		if (!deleteTarget) return;
		deleting = true;
		try {
			await api.delete(`/organizations/${organizationId}/incidents/${deleteTarget.id}`, {
				skipProjectId: true
			});
			toast.success('Successfully deleted the Incident');
			deleteTarget = null;
			expandedId = null;
			await loadIncidents();
		} catch (e: any) {
			if (e?.status !== 403) {
				toast.error(e?.message || 'Failed to delete the incident');
			}
		} finally {
			deleting = false;
		}
	}

	let newPostMortemOpen = $state(false);
	let newPostMortemIncident = $state<OrgIncident | null>(null);

	function openPostMortem(incident: OrgIncident) {
		if (incident.postMortemId) {
			goto(resolve(`/monitors/post-mortems/${incident.postMortemId}`));
		} else {
			newPostMortemIncident = incident;
			newPostMortemOpen = true;
		}
	}

	const backHandler = createSmartBackHandler({ fallbackPath: '/monitors?tab=status-pages' });
</script>

{#if notFound}
	<ErrorDisplay
		status={404}
		title="Status page not found"
		description="This status page does not exist or belongs to a different organization."
		onBack={backHandler}
		backLabel="Back to Status Pages"
	/>
{:else}
	<div class="space-y-4">
		<PageHeader
			title={statusPageName ? `Incidents · ${statusPageName}` : 'Incidents'}
			description="Everything the public Past Incidents section of this status page shows. Post public timeline updates, name incidents, and attach post-mortems."
			onBack={backHandler}
		>
			{#snippet actions()}
				{#if canManage}
					<Button variant="success" size="sm" onclick={() => (recordOpen = true)}>
						<Megaphone class="mr-2 h-4 w-4" />
						Record Incident
					</Button>
				{/if}
			{/snippet}
		</PageHeader>

		{#if loading}
			<div class="flex h-48 items-center justify-center">
				<LoadingCircle size="xlg" />
			</div>
		{:else if error}
			<div class="rounded-md border py-16 text-center text-red-500">{error}</div>
		{:else if incidents.length === 0}
			<div class="rounded-md border py-16 text-center text-sm text-muted-foreground">
				No incidents recorded on this status page yet.
			</div>
		{:else}
			<div class="space-y-3">
				{#each incidents as incident (incident.id)}
					<div class="rounded-md border">
						<button
							type="button"
							class="flex w-full cursor-pointer items-center gap-2 p-3 text-left"
							onclick={() => toggleExpand(incident)}
						>
							{#if expandedId === incident.id}
								<ChevronDown class="size-4 shrink-0 text-muted-foreground" />
							{:else}
								<ChevronRight class="size-4 shrink-0 text-muted-foreground" />
							{/if}
							<div class="min-w-0 flex-1">
								<div class="truncate text-sm font-medium">{incidentDisplayTitle(incident)}</div>
								<div class="text-xs text-muted-foreground">
									{formatDateTime(incident.startedAt, { format: 'short' })}
									{#if incident.statusPageId}
										· manual
									{:else if incident.checkName}
										· {incident.checkName}
									{/if}
									{#if incident.updatesCount}
										· {incident.updatesCount} update{incident.updatesCount === 1 ? '' : 's'}
									{/if}
								</div>
							</div>
							{#if incident.resolvedAt}
								<StatusPill tone="success" label="Resolved" />
							{:else}
								<StatusPill tone="danger" label="Ongoing" />
							{/if}
						</button>

						{#if expandedId === incident.id}
							<div class="space-y-4 border-t p-3">
								<div class="flex flex-wrap items-center gap-2">
									{#if canManage}
										{#if editingTitle}
											<Input
												bind:value={titleDraft}
												placeholder="Public incident title"
												class="h-8 w-full sm:w-auto sm:flex-1"
												disabled={savingTitle}
											/>
											<Button
												size="sm"
												variant="outline"
												onclick={() => saveTitle(incident)}
												disabled={savingTitle}
											>
												<Check class="mr-2 h-4 w-4" />
												Save
											</Button>
										{:else}
											<Button size="sm" variant="outline" onclick={() => startTitleEdit(incident)}>
												<Pencil class="mr-2 h-4 w-4" />
												{incident.title ? 'Rename' : 'Set title'}
											</Button>
										{/if}
									{/if}
									<Button size="sm" variant="outline" onclick={() => openPostMortem(incident)}>
										<FileText class="mr-2 h-4 w-4" />
										{incident.postMortemId ? 'View post-mortem' : 'Write post-mortem'}
									</Button>
									{#if canManage && incident.statusPageId && !incident.resolvedAt}
										<Button
											size="sm"
											variant="outline"
											onclick={() => postUpdate(incident, 'resolved')}
											disabled={posting}
										>
											<Check class="mr-2 h-4 w-4" />
											Resolve
										</Button>
									{/if}
									{#if canManage && incident.statusPageId}
										<Button
											size="sm"
											variant="destructiveOutline"
											onclick={() => (deleteTarget = incident)}
										>
											<Trash2 class="mr-2 h-4 w-4" />
											Delete
										</Button>
									{/if}
								</div>

								{#if canManage}
									<div class="flex flex-col gap-2 sm:flex-row sm:items-start">
										<Select.Root type="single" bind:value={updateStatus} disabled={posting}>
											<Select.Trigger class="w-full shrink-0 sm:w-[150px]">
												{statusLabel}
											</Select.Trigger>
											<Select.Content>
												{#each statusOptions.filter((o) => o.value !== 'resolved' || !!incident.statusPageId || !!incident.resolvedAt) as option (option.value)}
													<Select.Item value={option.value}>{option.label}</Select.Item>
												{/each}
											</Select.Content>
										</Select.Root>
										<div class="flex min-w-0 flex-1 gap-2">
											<Input
												bind:value={updateMessage}
												placeholder="Public update message..."
												class="min-w-0 flex-1"
												disabled={posting}
												onkeydown={(e) => {
													if (e.key === 'Enter') postUpdate(incident);
												}}
											/>
											<Button
												size="sm"
												class="h-9 shrink-0"
												onclick={() => postUpdate(incident)}
												disabled={posting}
											>
												<Send class="h-3.5 w-3.5" />
											</Button>
										</div>
									</div>
								{/if}

								{#if updatesLoading}
									<div class="flex justify-center py-4"><LoadingCircle size="md" /></div>
								{:else if updates.length === 0}
									<p class="text-sm text-muted-foreground">No updates yet.</p>
								{:else}
									<div class="space-y-3">
										{#each updates as update (update.id)}
											<div class="group flex items-start gap-2 text-sm">
												<div class="min-w-0 flex-1">
													<span class="font-semibold capitalize">{update.status}</span>
													{#if update.message}
														<span class="text-foreground/90"> - {update.message}</span>
													{/if}
													<div class="text-xs text-muted-foreground">
														{formatDateTime(update.createdAt, { format: 'short' })}
													</div>
												</div>
												{#if canManage}
													<Button
														variant="ghost"
														size="icon-sm"
														class="text-destructive opacity-60 hover:text-destructive hover:opacity-100 sm:opacity-0 sm:group-hover:opacity-100"
														title="Delete update"
														onclick={() => (deleteUpdateTarget = { incident, update })}
													>
														<Trash2 class="h-3.5 w-3.5" />
													</Button>
												{/if}
											</div>
										{/each}
									</div>
								{/if}
							</div>
						{/if}
					</div>
				{/each}
			</div>

			<PaginationFooter
				{currentPage}
				{totalPages}
				{pageSize}
				totalItems={total}
				onPageChange={(page) => {
					currentPage = page;
					expandedId = null;
					loadIncidents();
				}}
				onPageSizeChange={(size) => {
					pageSize = size;
					currentPage = 1;
					expandedId = null;
					loadIncidents();
				}}
				{loading}
				itemLabel="incident"
			/>
		{/if}
	</div>

	<RecordIncidentDialog
		bind:open={recordOpen}
		statusPage={statusPageName ? { id: parseInt(data.pageId, 10), name: statusPageName } : null}
		onSaved={() => loadIncidents()}
	/>

	<NewPostMortemDialog
		bind:open={newPostMortemOpen}
		incidentId={newPostMortemIncident?.id ?? null}
		incidentLabel={newPostMortemIncident ? incidentDisplayTitle(newPostMortemIncident) : ''}
	/>

	<ConfirmDeleteDialog
		bind:open={() => deleteUpdateTarget !== null, (v) => !v && (deleteUpdateTarget = null)}
		entity="Update"
		description={`This removes the "${deleteUpdateTarget?.update.status}" update from the public timeline.`}
		loading={deletingUpdate}
		onConfirm={deleteUpdate}
	/>

	<ConfirmDeleteDialog
		bind:open={() => deleteTarget !== null, (v) => !v && (deleteTarget = null)}
		entity="Incident"
		description={`This removes "${deleteTarget ? incidentDisplayTitle(deleteTarget) : ''}" and its updates from the status page. A linked post-mortem is kept.`}
		loading={deleting}
		onConfirm={deleteIncident}
	/>
{/if}
