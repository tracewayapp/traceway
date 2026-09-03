<script lang="ts">
	import { ArrowUpRight } from '@lucide/svelte';
	import { Badge } from '$lib/components/ui/badge';
	import { TracewayTableHeader } from '$lib/components/ui/traceway-table-header';
	import * as Table from '$lib/components/ui/table';
	import FrameworkIcon from '$lib/components/framework-icon.svelte';
	import PageHeader from '$lib/components/traceway/page-header.svelte';
	import TableContainer from '$lib/components/traceway/table-container.svelte';
	import CountPill from '../count-pill.svelte';
	import { authState } from '$lib/state/auth.svelte';
	import { organizationContext } from '$lib/state/organization-context.svelte';
	import { getFrameworkLabel, type Project } from '$lib/state/projects.svelte';
	import { formatRelativeTimeAgo } from '$lib/utils/formatters';
	import { createRowClickHandler } from '$lib/utils/navigation';

	const projects = $derived(organizationContext.projects);

	function projectRole(project: Project): string {
		const organizationId = organizationContext.organizationId;
		return (
			project.role ||
			(organizationId === null ? null : authState.getRoleForOrganization(organizationId)) ||
			'user'
		);
	}

	function roleLabel(role: string): string {
		switch (role) {
			case 'owner':
				return 'Owner';
			case 'admin':
				return 'Admin';
			case 'readonly':
				return 'Read only';
			default:
				return 'Member';
		}
	}
</script>

<div class="space-y-4">
	<PageHeader
		title="Projects"
		description="Every project in the organization, with its framework and your effective access level."
	>
		{#snippet trailing()}
			<CountPill count={projects.length} />
		{/snippet}
	</PageHeader>

	{#if projects.length === 0}
		<div class="rounded-md border py-16 text-center text-sm text-muted-foreground">
			No projects have been added to this organization yet.
		</div>
	{:else}
		<TableContainer minWidth="680px">
			<Table.Root>
				<Table.Header>
					<Table.Row>
						<TracewayTableHeader label="Project" />
						<TracewayTableHeader label="Framework" class="w-[200px]" />
						<TracewayTableHeader label="Access" class="w-[130px]" />
						<TracewayTableHeader label="Added" class="w-[150px]" align="right" />
						<TracewayTableHeader label="" class="w-[48px]" />
					</Table.Row>
				</Table.Header>
				<Table.Body>
					{#each projects as project (project.id)}
						<Table.Row
							class="group cursor-pointer"
							onclick={createRowClickHandler(`/?projectId=${project.id}`)}
						>
							<Table.Cell>
								<div class="flex items-center gap-3">
									<div
										class="flex size-9 shrink-0 items-center justify-center rounded-md border bg-muted/30"
									>
										<FrameworkIcon framework={project.framework} class="size-5" />
									</div>
									<span class="font-medium group-hover:text-primary">{project.name}</span>
								</div>
							</Table.Cell>
							<Table.Cell class="text-sm text-muted-foreground">
								{getFrameworkLabel(project.framework)}
							</Table.Cell>
							<Table.Cell>
								<Badge variant="outline">{roleLabel(projectRole(project))}</Badge>
							</Table.Cell>
							<Table.Cell class="text-right text-sm text-muted-foreground">
								{formatRelativeTimeAgo(project.createdAt)}
							</Table.Cell>
							<Table.Cell>
								<ArrowUpRight class="size-4 text-muted-foreground group-hover:text-foreground" />
							</Table.Cell>
						</Table.Row>
					{/each}
				</Table.Body>
			</Table.Root>
		</TableContainer>
	{/if}
</div>
