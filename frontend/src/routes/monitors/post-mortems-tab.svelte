<script lang="ts">
	import { untrack } from 'svelte';
	import { SvelteURLSearchParams } from 'svelte/reactivity';
	import * as Table from '$lib/components/ui/table';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { TableEmptyState } from '$lib/components/ui/table-empty-state';
	import { PaginationFooter } from '$lib/components/ui/pagination-footer';
	import { SearchBar } from '$lib/components/ui/search-bar';
	import { Badge } from '$lib/components/ui/badge';
	import EmptyState from '$lib/components/traceway/empty-state.svelte';
	import TableContainer from '$lib/components/traceway/table-container.svelte';
	import NewPostMortemDialog from './new-post-mortem-dialog.svelte';
	import { FileText, Link2, X } from '@lucide/svelte';
	import { api } from '$lib/api';
	import { resolve } from '$app/paths';
	import { createRowClickHandler } from '$lib/utils/navigation';
	import { formatRelativeTimeAgo } from '$lib/utils/formatters';
	import { projectsState } from '$lib/state/projects.svelte';
	import { authState } from '$lib/state/auth.svelte';
	import { type PostMortemListItem } from '$lib/state/monitors.svelte';

	let items = $state<PostMortemListItem[]>([]);
	let loading = $state(true);
	let error = $state('');
	let searchQuery = $state('');
	let tagFilters = $state<string[]>([]);
	let currentPage = $state(1);
	let pageSize = $state(25);
	let total = $state(0);
	let totalPages = $state(0);

	const organizationId = $derived(projectsState.currentProject?.organizationId ?? null);
	const canWrite = $derived(
		organizationId !== null && authState.canWriteOrganization(organizationId)
	);

	let createOpen = $state(false);

	export function openCreate() {
		createOpen = true;
	}

	let loadSeq = 0;

	async function load() {
		if (organizationId === null) return;
		const seq = ++loadSeq;
		loading = true;
		error = '';
		try {
			const params = new SvelteURLSearchParams();
			if (searchQuery.trim()) params.set('search', searchQuery.trim());
			for (const tag of tagFilters) params.append('tag', tag);
			params.set('page', String(currentPage));
			params.set('pageSize', String(pageSize));
			const res = await api.get(`/organizations/${organizationId}/post-mortems?${params}`, {
				skipProjectId: true
			});
			if (seq !== loadSeq) return;
			items = res.data || [];
			total = res.pagination?.total || 0;
			totalPages = res.pagination?.totalPages || 0;
		} catch (e) {
			if (seq !== loadSeq) return;
			error = e instanceof Error ? e.message : 'Failed to load post-mortems';
		} finally {
			if (seq === loadSeq) loading = false;
		}
	}

	$effect(() => {
		void organizationId;
		untrack(() => {
			searchQuery = '';
			tagFilters = [];
			currentPage = 1;
			load();
		});
	});

	function handleSearch() {
		currentPage = 1;
		load();
	}

	function toggleTag(tag: string) {
		tagFilters = tagFilters.includes(tag)
			? tagFilters.filter((t) => t !== tag)
			: [...tagFilters, tag];
		currentPage = 1;
		load();
	}

	const hasFilters = $derived(searchQuery.trim() !== '' || tagFilters.length > 0);
</script>

<div class="space-y-4">
	<SearchBar
		placeholder="Search post-mortems..."
		bind:value={searchQuery}
		onSearch={handleSearch}
		disabled={loading}
	/>

	{#if tagFilters.length > 0}
		<div class="flex flex-wrap items-center gap-1.5">
			{#each tagFilters as tag (tag)}
				<span
					class="inline-flex items-center gap-1 rounded-full bg-muted px-2 py-0.5 font-mono text-xs"
				>
					<span class="text-muted-foreground">tag</span>
					<span>=</span>
					<span>{tag}</span>
					<button
						type="button"
						aria-label="Remove filter"
						class="ml-1 cursor-pointer text-muted-foreground hover:text-foreground"
						onclick={() => toggleTag(tag)}
					>
						<X class="h-3 w-3" />
					</button>
				</span>
			{/each}
		</div>
	{/if}

	{#if loading}
		<div class="flex h-48 items-center justify-center">
			<LoadingCircle size="xlg" />
		</div>
	{:else if error}
		<div class="rounded-md border py-16 text-center text-red-500">{error}</div>
	{:else if items.length === 0 && !hasFilters}
		{#if canWrite}
			<EmptyState
				message="No post-mortems yet. Write one after your next incident so the lessons stick around."
				actionLabel="Write your first Post-Mortem"
				onAction={openCreate}
			/>
		{:else}
			<EmptyState message="No post-mortems yet." />
		{/if}
	{:else}
		<TableContainer>
			<Table.Root>
				<Table.Header>
					<Table.Row>
						<Table.Head>Title</Table.Head>
						<Table.Head>Tags</Table.Head>
						<Table.Head class="w-[110px]">Incident</Table.Head>
						<Table.Head class="w-[140px]">Updated</Table.Head>
					</Table.Row>
				</Table.Header>
				<Table.Body>
					{#if items.length === 0}
						<TableEmptyState colspan={4} message="No post-mortems match your search." />
					{:else}
						{#each items as item (item.id)}
							<Table.Row
								class="cursor-pointer"
								onclick={createRowClickHandler(resolve(`/monitors/post-mortems/${item.id}`))}
							>
								<Table.Cell>
									<div class="flex items-center gap-2 font-medium">
										<FileText class="size-4 shrink-0 text-muted-foreground" />
										{item.title}
									</div>
								</Table.Cell>
								<Table.Cell>
									{#if item.tags.length > 0}
										<div class="flex flex-wrap gap-1">
											{#each item.tags as tag (tag)}
												<button
													type="button"
													class="cursor-pointer"
													title={`Filter by tag "${tag}"`}
													onclick={(e) => {
														e.stopPropagation();
														toggleTag(tag);
													}}
												>
													<Badge variant={tagFilters.includes(tag) ? 'default' : 'secondary'}>
														{tag}
													</Badge>
												</button>
											{/each}
										</div>
									{:else}
										<span class="text-muted-foreground">—</span>
									{/if}
								</Table.Cell>
								<Table.Cell>
									{#if item.incidentId}
										<span class="flex items-center gap-1 text-sm text-muted-foreground">
											<Link2 class="size-3.5" /> Linked
										</span>
									{:else}
										<span class="text-muted-foreground">—</span>
									{/if}
								</Table.Cell>
								<Table.Cell class="text-muted-foreground">
									<div>{formatRelativeTimeAgo(item.updatedAt)}</div>
									{#if item.updatedByName || item.createdByName}
										<div class="text-xs">by {item.updatedByName || item.createdByName}</div>
									{/if}
								</Table.Cell>
							</Table.Row>
						{/each}
					{/if}
				</Table.Body>
			</Table.Root>
		</TableContainer>

		<PaginationFooter
			{currentPage}
			{totalPages}
			{pageSize}
			totalItems={total}
			onPageChange={(page) => {
				currentPage = page;
				load();
			}}
			onPageSizeChange={(size) => {
				pageSize = size;
				currentPage = 1;
				load();
			}}
			{loading}
			itemLabel="post-mortem"
		/>
	{/if}
</div>

<NewPostMortemDialog bind:open={createOpen} />
