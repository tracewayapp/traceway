<script lang="ts">
	import { api } from '$lib/api';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import * as Table from '$lib/components/ui/table';
	import { TracewayTableHeader } from '$lib/components/ui/traceway-table-header';
	import EmptyState from '$lib/components/traceway/empty-state.svelte';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Checkbox } from '$lib/components/ui/checkbox';
	import { ErrorAlert } from '$lib/components/ui/error-alert';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { projectsState } from '$lib/state/projects.svelte';
	import { authState } from '$lib/state/auth.svelte';
	import type { SyntheticCheck } from '$lib/state/monitors.svelte';
	import { Check, ExternalLink, Globe, ImageUp, Pencil, Plus, Trash2 } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';

	interface StatusPage {
		id: number;
		organizationId: number;
		slug: string;
		name: string;
		isPublic: boolean;
		checkIds: number[];
		description: string;
		logoKey: string;
		customDomain: string;
	}

	const organizationId = $derived(projectsState.currentProject?.organizationId ?? null);

	let pages = $state<StatusPage[]>([]);
	let checks = $state<SyntheticCheck[]>([]);
	let loading = $state(true);

	let dialogOpen = $state(false);
	let editing = $state<StatusPage | null>(null);
	let saving = $state(false);
	let dialogError = $state('');
	let pageName = $state('');
	let pageSlug = $state('');
	let pagePublic = $state(true);
	let pageDescription = $state('');
	let pageCustomDomain = $state('');
	let selectedCheckIds = $state<number[]>([]);
	let logoFile = $state<File | null>(null);
	let logoInput = $state<HTMLInputElement | null>(null);

	let deleteTarget = $state<StatusPage | null>(null);
	let deleting = $state(false);

	async function loadAll() {
		if (!organizationId) return;
		loading = true;
		try {
			const [pagesResponse, checksResponse] = await Promise.all([
				api.get(`/organizations/${organizationId}/status-pages`, { skipProjectId: true }),
				api.get('/synthetics/checks', { projectId: projectsState.currentProjectId ?? undefined })
			]);
			pages = pagesResponse.statusPages || [];
			checks = checksResponse.checks || [];
		} catch (e) {
			console.error('Failed to load status pages:', e);
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		if (organizationId) {
			loadAll();
		}
	});

	export function openCreate() {
		editing = null;
		pageName = '';
		pageSlug = '';
		pagePublic = true;
		pageDescription = '';
		pageCustomDomain = '';
		selectedCheckIds = [];
		logoFile = null;
		dialogError = '';
		dialogOpen = true;
	}

	function openEdit(page: StatusPage) {
		editing = page;
		pageName = page.name;
		pageSlug = page.slug;
		pagePublic = page.isPublic;
		pageDescription = page.description || '';
		pageCustomDomain = page.customDomain || '';
		selectedCheckIds = [...(page.checkIds || [])];
		logoFile = null;
		dialogError = '';
		dialogOpen = true;
	}

	function toggleCheck(checkId: number, checked: boolean) {
		if (checked) {
			selectedCheckIds = [...selectedCheckIds, checkId];
		} else {
			selectedCheckIds = selectedCheckIds.filter((id) => id !== checkId);
		}
	}

	async function uploadLogo(pageId: number) {
		if (!logoFile) return;
		const body = await logoFile.arrayBuffer();
		const response = await fetch(
			`/api/organizations/${organizationId}/status-pages/${pageId}/logo`,
			{
				method: 'POST',
				headers: { Authorization: `Bearer ${authState.token}` },
				body
			}
		);
		if (!response.ok) {
			const data = await response.json().catch(() => ({}));
			throw new Error(data.error || 'Failed to upload the logo');
		}
	}

	async function savePage() {
		saving = true;
		dialogError = '';
		try {
			const payload = {
				name: pageName.trim(),
				slug: pageSlug.trim().toLowerCase(),
				isPublic: pagePublic,
				checkIds: selectedCheckIds,
				description: pageDescription.trim(),
				customDomain: pageCustomDomain.trim().toLowerCase()
			};
			let savedId: number;
			if (editing) {
				const response = await api.put(
					`/organizations/${organizationId}/status-pages/${editing.id}`,
					payload,
					{ skipProjectId: true }
				);
				savedId = response.id;
			} else {
				const response = await api.post(`/organizations/${organizationId}/status-pages`, payload, {
					skipProjectId: true
				});
				savedId = response.id;
			}
			if (logoFile) {
				await uploadLogo(savedId);
			}
			toast.success(
				editing ? 'Successfully updated the Status Page' : 'Successfully created the Status Page',
				{ position: 'top-center' }
			);
			dialogOpen = false;
			loadAll();
		} catch (e) {
			dialogError = e instanceof Error ? e.message : 'Failed to save the status page';
		} finally {
			saving = false;
		}
	}

	async function deletePage() {
		if (!deleteTarget) return;
		deleting = true;
		try {
			await api.delete(`/organizations/${organizationId}/status-pages/${deleteTarget.id}`, {
				skipProjectId: true
			});
			toast.success('Successfully deleted the Status Page', { position: 'top-center' });
			deleteTarget = null;
			loadAll();
		} catch (e: any) {
			if (e?.status !== 403) {
				toast.error(e.message || 'Failed to delete the status page', { position: 'top-center' });
			}
		} finally {
			deleting = false;
		}
	}
</script>

{#if loading}
	<div class="flex h-48 items-center justify-center">
		<LoadingCircle size="xlg" />
	</div>
{:else if pages.length === 0}
	<EmptyState
		message="No status pages yet. Publish the uptime of your monitors on a public page."
		actionLabel="Create your first Status Page"
		onAction={openCreate}
	/>
{:else}
	<div class="overflow-hidden rounded-md border">
		<Table.Root>
			<Table.Header>
				<Table.Row>
					<TracewayTableHeader label="Name" />
					<TracewayTableHeader label="Public URL" class="w-[220px]" />
					<TracewayTableHeader label="Custom domain" class="w-[200px]" />
					<TracewayTableHeader label="Monitors" class="w-[100px]" />
					<TracewayTableHeader label="Visibility" class="w-[110px]" />
					<TracewayTableHeader label="" align="right" class="w-[90px]" />
				</Table.Row>
			</Table.Header>
			<Table.Body>
				{#each pages as page}
					<Table.Row class="group hover:bg-muted/50">
						<Table.Cell class="font-medium">
							{page.name}
							{#if page.description}
								<div class="mt-0.5 text-xs font-normal text-muted-foreground">
									{page.description}
								</div>
							{/if}
						</Table.Cell>
						<Table.Cell>
							<a
								class="inline-flex items-center gap-1 font-mono text-xs text-primary underline-offset-2 hover:underline"
								href={`/status/${page.slug}`}
								target="_blank"
								rel="noopener"
							>
								/status/{page.slug}
								<ExternalLink class="h-3 w-3" />
							</a>
						</Table.Cell>
						<Table.Cell class="font-mono text-xs text-muted-foreground">
							{page.customDomain || '—'}
						</Table.Cell>
						<Table.Cell class="text-sm text-muted-foreground tabular-nums">
							{(page.checkIds || []).length}
						</Table.Cell>
						<Table.Cell>
							{#if page.isPublic}
								<span
									class="inline-flex items-center gap-1 rounded-full bg-green-500/15 px-2 py-0.5 text-xs font-medium text-green-600 dark:text-green-400"
								>
									<Globe class="h-3 w-3" />
									Public
								</span>
							{:else}
								<span
									class="inline-flex items-center rounded-full bg-muted px-2 py-0.5 text-xs font-medium text-muted-foreground"
								>
									Private
								</span>
							{/if}
						</Table.Cell>
						<Table.Cell class="text-right">
							<div class="flex justify-end gap-1">
								<Button variant="ghost" size="icon-sm" onclick={() => openEdit(page)} title="Edit">
									<Pencil class="h-4 w-4" />
								</Button>
								<Button
									variant="ghost"
									size="icon-sm"
									onclick={() => (deleteTarget = page)}
									title="Delete"
								>
									<Trash2 class="h-4 w-4" />
								</Button>
							</div>
						</Table.Cell>
					</Table.Row>
				{/each}
			</Table.Body>
		</Table.Root>
	</div>
{/if}

<AlertDialog.Root open={dialogOpen} onOpenChange={(v) => (dialogOpen = v)}>
	<AlertDialog.Content
		class="max-h-[90vh] overflow-y-auto sm:max-w-xl"
		interactOutsideBehavior={saving ? 'ignore' : 'close'}
	>
		<AlertDialog.Header>
			<AlertDialog.Title>{editing ? 'Edit Status Page' : 'New Status Page'}</AlertDialog.Title>
			<AlertDialog.Description>
				Pick the monitors to show, the public URL, and the branding.
			</AlertDialog.Description>
		</AlertDialog.Header>

		<form
			onsubmit={(e) => {
				e.preventDefault();
				savePage();
			}}
			class="space-y-4"
		>
			<ErrorAlert error={dialogError} />
			<div class="grid grid-cols-2 gap-4">
				<div class="space-y-2">
					<Label for="status-page-name">Title</Label>
					<Input
						id="status-page-name"
						bind:value={pageName}
						placeholder="Acme Status"
						disabled={saving}
					/>
				</div>
				<div class="space-y-2">
					<Label for="status-page-slug">Slug</Label>
					<div class="flex items-center gap-2">
						<span class="text-sm text-muted-foreground">/status/</span>
						<Input
							id="status-page-slug"
							bind:value={pageSlug}
							placeholder="acme"
							class="flex-1"
							disabled={saving}
						/>
					</div>
				</div>
			</div>
			<div class="space-y-2">
				<Label for="status-page-description">Description (optional)</Label>
				<Input
					id="status-page-description"
					bind:value={pageDescription}
					placeholder="Live availability of Acme services"
					disabled={saving}
				/>
			</div>
			<div class="space-y-2">
				<Label>Logo (optional, PNG or JPEG)</Label>
				<div class="flex items-center gap-3">
					{#if editing?.logoKey && !logoFile}
						<img
							src={`/api/status/${editing.slug}/logo`}
							alt="Current logo"
							class="h-9 w-9 rounded border object-contain"
						/>
					{/if}
					<input
						bind:this={logoInput}
						type="file"
						accept="image/png,image/jpeg"
						class="hidden"
						onchange={(e) => (logoFile = (e.currentTarget as HTMLInputElement).files?.[0] ?? null)}
					/>
					<Button
						variant="outline"
						size="sm"
						type="button"
						onclick={() => logoInput?.click()}
						disabled={saving}
					>
						<ImageUp class="mr-1.5 h-4 w-4" />
						{logoFile ? logoFile.name : editing?.logoKey ? 'Replace logo' : 'Upload logo'}
					</Button>
				</div>
			</div>
			<div class="space-y-2">
				<Label for="status-page-domain">Custom domain (optional)</Label>
				<Input
					id="status-page-domain"
					bind:value={pageCustomDomain}
					placeholder="status.acme.com"
					class="font-mono text-sm"
					disabled={saving}
				/>
				<p class="text-xs text-muted-foreground">
					Point a CNAME at this Traceway instance and terminate TLS for it at your reverse proxy.
					Visitors landing on that host are routed to this status page.
				</p>
			</div>
			<label class="flex cursor-pointer items-center gap-2 text-sm">
				<Checkbox
					checked={pagePublic}
					onCheckedChange={(checked) => (pagePublic = checked === true)}
					disabled={saving}
				/>
				Publicly accessible
			</label>
			<div class="space-y-2">
				<Label>Monitors (from this project)</Label>
				{#if checks.length === 0}
					<p class="text-sm text-muted-foreground">Create a monitor first.</p>
				{:else}
					<div class="max-h-48 space-y-1 overflow-y-auto rounded-md border p-2">
						{#each checks as check}
							<label
								class="flex cursor-pointer items-center gap-2 rounded p-1.5 text-sm hover:bg-muted/50"
							>
								<Checkbox
									checked={selectedCheckIds.includes(check.id)}
									onCheckedChange={(checked) => toggleCheck(check.id, checked === true)}
									disabled={saving}
								/>
								{check.name}
								<span class="font-mono text-xs text-muted-foreground uppercase"
									>{check.checkType}</span
								>
							</label>
						{/each}
					</div>
					{#if editing}
						<p class="text-xs text-muted-foreground">
							Monitors from other projects stay selected even though only this project's monitors
							are listed here.
						</p>
					{/if}
				{/if}
			</div>
		</form>

		<AlertDialog.Footer>
			<AlertDialog.Cancel disabled={saving}>Cancel</AlertDialog.Cancel>
			<Button variant={editing ? 'default' : 'success'} onclick={savePage} disabled={saving}>
				{#if editing}
					<Check class="mr-2 h-4 w-4" />
					{saving ? 'Updating...' : 'Update Status Page'}
				{:else}
					<Plus class="mr-2 h-4 w-4" />
					{saving ? 'Creating...' : 'New Status Page'}
				{/if}
			</Button>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>

<AlertDialog.Root open={deleteTarget !== null} onOpenChange={(v) => !v && (deleteTarget = null)}>
	<AlertDialog.Content interactOutsideBehavior={deleting ? 'ignore' : 'close'}>
		<AlertDialog.Header>
			<AlertDialog.Title>Delete Status Page</AlertDialog.Title>
			<AlertDialog.Description>
				"{deleteTarget?.name}" at /status/{deleteTarget?.slug} will stop working immediately.
			</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel disabled={deleting}>Cancel</AlertDialog.Cancel>
			<Button variant="destructive" onclick={deletePage} disabled={deleting}>
				<Trash2 class="mr-2 h-4 w-4" />
				{deleting ? 'Deleting...' : 'Delete Status Page'}
			</Button>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
