<script lang="ts">
	import { toast } from 'svelte-sonner';
	import {
		Bookmark,
		BookmarkCheck,
		Check,
		ChevronDown,
		Pencil,
		Plus,
		Trash2
	} from '@lucide/svelte';
	import { api } from '$lib/api';
	import { projectsState } from '$lib/state/projects.svelte';
	import { getErrorMessage } from '$lib/utils/errors';
	import * as Popover from '$lib/components/ui/popover';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Checkbox } from '$lib/components/ui/checkbox';
	import { ErrorAlert } from '$lib/components/ui/error-alert';
	import FormField from '$lib/components/traceway/form-field.svelte';
	import ConfirmDeleteDialog from '$lib/components/traceway/confirm-delete-dialog.svelte';
	import {
		describeSavedFilter,
		hasTimeRange,
		matchesSavedFilter,
		normalizeQuery,
		type SavedFilter,
		type SavedFilterPage
	} from '$lib/utils/saved-filters';

	let {
		page,
		currentQuery,
		timeRangeLabel,
		namePlaceholder = 'Slow telegrams on node 7',
		onApply
	}: {
		page: SavedFilterPage;
		// The page's filters as a URL query string, time range included.
		currentQuery: string;
		timeRangeLabel: string;
		namePlaceholder?: string;
		onApply: (filter: SavedFilter) => void;
	} = $props();

	let filters = $state<SavedFilter[]>([]);
	let loadError = $state('');
	let open = $state(false);

	let saveOpen = $state(false);
	let editing = $state<SavedFilter | null>(null);
	let deleting = $state<SavedFilter | null>(null);
	let deleteOpen = $state(false);
	let name = $state('');
	let includeTime = $state(false);
	let replaceFilters = $state(false);
	let dialogError = $state('');
	let busy = $state(false);

	const canWrite = $derived(projectsState.canWriteCurrentProject);
	const active = $derived(filters.find((f) => matchesSavedFilter(currentQuery, f)) ?? null);
	const nothingToSave = $derived(normalizeQuery(currentQuery, page, false) === '');

	const savePreview = $derived(
		describeSavedFilter(normalizeQuery(currentQuery, page, includeTime), page)
	);

	async function loadFilters() {
		try {
			const response = (await api.get(`/saved-filters?page=${page}`, {
				projectId: projectsState.currentProjectId ?? undefined
			})) as { data: SavedFilter[] };
			filters = response.data ?? [];
			loadError = '';
		} catch (e) {
			loadError = getErrorMessage(e) || 'Failed to load saved filters';
		}
	}

	$effect(() => {
		void projectsState.currentProjectId;
		loadFilters();
	});

	function openSave() {
		name = '';
		includeTime = false;
		dialogError = '';
		editing = null;
		open = false;
		saveOpen = true;
	}

	function openEdit(filter: SavedFilter) {
		name = filter.name;
		replaceFilters = false;
		includeTime = hasTimeRange(filter.query);
		dialogError = '';
		editing = filter;
		open = false;
		saveOpen = true;
	}

	function openDelete(filter: SavedFilter) {
		deleting = filter;
		dialogError = '';
		open = false;
		deleteOpen = true;
	}

	async function submit() {
		busy = true;
		dialogError = '';
		try {
			if (editing) {
				const body: { name: string; query?: string } = { name };
				if (replaceFilters) body.query = normalizeQuery(currentQuery, page, includeTime);
				await api.put(`/saved-filters/${editing.id}`, body, {
					projectId: projectsState.currentProjectId ?? undefined
				});
				toast.success('Successfully updated the Saved Filter', { position: 'top-center' });
			} else {
				await api.post(
					'/saved-filters',
					{ page, name, query: normalizeQuery(currentQuery, page, includeTime) },
					{ projectId: projectsState.currentProjectId ?? undefined }
				);
				toast.success('Successfully created the Saved Filter', { position: 'top-center' });
			}
			saveOpen = false;
			await loadFilters();
		} catch (e) {
			dialogError = getErrorMessage(e) || 'Failed to save the filter';
		} finally {
			busy = false;
		}
	}

	async function confirmDelete() {
		if (!deleting) return;
		busy = true;
		dialogError = '';
		try {
			await api.delete(`/saved-filters/${deleting.id}`, {
				projectId: projectsState.currentProjectId ?? undefined
			});
			toast.success('Successfully deleted the Saved Filter', { position: 'top-center' });
			deleteOpen = false;
			await loadFilters();
		} catch (e) {
			dialogError = getErrorMessage(e) || 'Failed to delete the filter';
		} finally {
			busy = false;
		}
	}

	function apply(filter: SavedFilter) {
		open = false;
		onApply(filter);
	}

	function handleNameKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter' && !busy) {
			e.preventDefault();
			submit();
		}
	}
</script>

<Popover.Root bind:open>
	<Popover.Trigger
		class="inline-flex h-8 max-w-64 items-center gap-1.5 rounded-md border bg-background px-3 text-xs font-medium shadow-xs transition-colors {active
			? 'border-primary/40 bg-primary/10 text-primary'
			: 'text-muted-foreground hover:border-foreground/40 hover:text-foreground'}"
		aria-label="Saved filters"
	>
		{#if active}
			<BookmarkCheck class="h-3 w-3 shrink-0" />
			<span class="truncate">{active.name}</span>
		{:else}
			<Bookmark class="h-3 w-3 shrink-0" />
			Saved filters
			{#if filters.length > 0}
				<span class="text-muted-foreground tabular-nums">{filters.length}</span>
			{/if}
		{/if}
		<ChevronDown class="h-3 w-3 shrink-0 opacity-60" />
	</Popover.Trigger>
	<Popover.Content align="end" class="w-96 p-0">
		<div class="border-b px-3 py-2">
			<p class="text-sm font-medium">Saved filters</p>
			<p class="text-xs text-muted-foreground">Shared with everyone in this project.</p>
		</div>
		{#if loadError}
			<p class="px-3 py-4 text-xs text-red-500">{loadError}</p>
		{:else if filters.length === 0}
			<p class="px-3 py-4 text-xs text-muted-foreground">
				No saved filters yet. Save the filters you use often to get back to them in one click.
			</p>
		{:else}
			<ul class="max-h-80 overflow-y-auto py-1">
				{#each filters as filter (filter.id)}
					{@const parts = describeSavedFilter(filter.query, page)}
					<li class="group flex items-start gap-1 px-1">
						<button
							type="button"
							class="flex min-w-0 flex-1 items-start gap-2 rounded-md px-2 py-1.5 text-left hover:bg-muted"
							onclick={() => apply(filter)}
						>
							<Check
								class="mt-0.5 h-3.5 w-3.5 shrink-0 {active?.id === filter.id
									? 'text-primary'
									: 'opacity-0'}"
							/>
							<span class="min-w-0">
								<span class="block truncate text-sm">{filter.name}</span>
								<span
									class="block truncate text-xs text-muted-foreground"
									title={parts.join(' · ')}
								>
									{parts.join(' · ')}
								</span>
								{#if filter.createdByName}
									<span class="block text-[11px] text-muted-foreground/80">
										by {filter.createdByName}
									</span>
								{/if}
							</span>
						</button>
						{#if canWrite}
							<div
								class="flex shrink-0 items-center gap-0.5 pt-1.5 opacity-100 md:opacity-0 md:group-focus-within:opacity-100 md:group-hover:opacity-100"
							>
								<button
									type="button"
									class="rounded p-1 text-muted-foreground hover:bg-muted hover:text-foreground"
									aria-label="Edit {filter.name}"
									title="Rename or replace"
									onclick={() => openEdit(filter)}
								>
									<Pencil class="h-3.5 w-3.5" />
								</button>
								<button
									type="button"
									class="rounded p-1 text-muted-foreground hover:bg-muted hover:text-red-500"
									aria-label="Delete {filter.name}"
									title="Delete"
									onclick={() => openDelete(filter)}
								>
									<Trash2 class="h-3.5 w-3.5" />
								</button>
							</div>
						{/if}
					</li>
				{/each}
			</ul>
		{/if}
		{#if canWrite}
			<div class="border-t p-2">
				<Button
					variant="ghost"
					size="sm"
					class="w-full justify-start"
					onclick={openSave}
					disabled={nothingToSave}
					title={nothingToSave ? 'Add a filter, a search or a chart first' : undefined}
				>
					<Plus class="h-4 w-4" />
					Save current filters
				</Button>
			</div>
		{/if}
	</Popover.Content>
</Popover.Root>

<AlertDialog.Root open={saveOpen} onOpenChange={(o) => (saveOpen = o)}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>{editing ? 'Edit saved filter' : 'Save filters'}</AlertDialog.Title>
			<AlertDialog.Description>
				Saved filters are shared with everyone in this project.
			</AlertDialog.Description>
		</AlertDialog.Header>
		<div class="flex flex-col gap-3">
			<FormField label="Name">
				<Input placeholder={namePlaceholder} bind:value={name} onkeydown={handleNameKeydown} />
			</FormField>
			{#if editing}
				<label class="flex cursor-pointer items-start gap-2 text-sm">
					<Checkbox
						checked={replaceFilters}
						onCheckedChange={(checked) => (replaceFilters = checked === true)}
						class="mt-0.5"
					/>
					<span>
						Replace its filters with the current ones
						<span class="block text-xs text-muted-foreground">
							Otherwise only the name changes.
						</span>
					</span>
				</label>
			{/if}
			{#if !editing || replaceFilters}
				<label class="flex cursor-pointer items-start gap-2 text-sm">
					<Checkbox
						checked={includeTime}
						onCheckedChange={(checked) => (includeTime = checked === true)}
						class="mt-0.5"
					/>
					<span>
						Include the time range ({timeRangeLabel})
						<span class="block text-xs text-muted-foreground">
							Without it, the filter applies to whatever range is selected.
						</span>
					</span>
				</label>
				<div class="rounded-md bg-muted px-3 py-2">
					<p class="mb-1 text-xs font-medium text-muted-foreground">Saves</p>
					<div class="flex flex-wrap gap-1">
						{#each savePreview as part, i (i)}
							<span class="rounded-full bg-background px-2 py-0.5 font-mono text-xs">{part}</span>
						{/each}
					</div>
				</div>
			{/if}
			<ErrorAlert error={dialogError || null} />
		</div>
		<AlertDialog.Footer>
			<Button variant="outline" onclick={() => (saveOpen = false)} disabled={busy}>Cancel</Button>
			{#if editing}
				<Button onclick={submit} disabled={busy}>
					<Check class="h-4 w-4" /> Update Saved Filter
				</Button>
			{:else}
				<Button variant="success" onclick={submit} disabled={busy}>
					<Plus class="h-4 w-4" /> New Saved Filter
				</Button>
			{/if}
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>

<ConfirmDeleteDialog
	bind:open={deleteOpen}
	entity="Saved Filter"
	description={deleting
		? `"${deleting.name}" will be removed for everyone in this project.`
		: undefined}
	error={dialogError || null}
	loading={busy}
	onConfirm={confirmDelete}
/>
