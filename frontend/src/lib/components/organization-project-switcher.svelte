<script lang="ts">
	import { Building2, Check, ChevronDown, Pencil, Plus } from '@lucide/svelte';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import { authState } from '$lib/state/auth.svelte';
	import { organizationContext } from '$lib/state/organization-context.svelte';
	import { projectsState, type Project } from '$lib/state/projects.svelte';
	import { gotoHref } from '$lib/utils/navigation';
	import FrameworkIcon from './framework-icon.svelte';

	interface Props {
		onAddProject: () => void;
		onEditProject: () => void;
	}

	let { onAddProject, onEditProject }: Props = $props();

	const groups = $derived.by(() => {
		const grouped = authState.organizations.map((organization) => ({
			organization,
			projects: projectsState.projects.filter(
				(project) => project.organizationId === organization.id
			)
		}));
		const knownOrganizationIds = new Set(
			authState.organizations.map((organization) => organization.id)
		);
		const ungroupedProjects = projectsState.projects.filter(
			(project) =>
				project.organizationId === null || !knownOrganizationIds.has(project.organizationId)
		);
		return { grouped, ungroupedProjects };
	});

	function selectProject(projectId: string) {
		gotoHref(`/?projectId=${projectId}`);
	}

	function selectOrganization(organizationId: number) {
		gotoHref(`/organization?organizationId=${organizationId}`);
	}
</script>

{#snippet projectItem(project: Project)}
	<DropdownMenu.Item
		onclick={() => selectProject(project.id)}
		class="flex cursor-pointer items-center justify-between py-2 pl-4"
	>
		<div class="flex min-w-0 items-center gap-2.5">
			<FrameworkIcon framework={project.framework} class="size-5 shrink-0" />
			<span class="truncate">{project.name}</span>
		</div>
		{#if !organizationContext.active && project.id === projectsState.currentProjectId}
			<Check class="size-4 shrink-0" />
		{/if}
	</DropdownMenu.Item>
{/snippet}

<DropdownMenu.Root>
	<DropdownMenu.Trigger
		class="flex max-w-[min(70vw,32rem)] min-w-0 items-center gap-2 rounded-md px-2 py-1 text-lg font-semibold transition-colors hover:bg-accent hover:text-accent-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
		aria-label="Switch organization or project"
	>
		{#if organizationContext.active}
			<Building2 class="size-5 shrink-0 text-muted-foreground" />
			<span class="truncate">{organizationContext.organization?.name || 'Select organization'}</span
			>
		{:else}
			{#if projectsState.currentProject}
				<FrameworkIcon framework={projectsState.currentProject.framework} class="size-6 shrink-0" />
			{/if}
			<span class="truncate">{projectsState.currentProject?.name || 'Select project'}</span>
		{/if}
		<ChevronDown class="size-4 shrink-0" />
	</DropdownMenu.Trigger>

	<DropdownMenu.Content align="start" class="w-[min(22rem,calc(100vw-1rem))]">
		{#each groups.grouped as group, index (group.organization.id)}
			{#if index > 0}
				<DropdownMenu.Separator />
			{/if}
			<DropdownMenu.Group>
				{#if group.projects.length > 1}
					<DropdownMenu.Item
						onclick={() => selectOrganization(group.organization.id)}
						class="cursor-pointer py-2"
					>
						<Building2 class="size-5 shrink-0" />
						<div class="min-w-0 flex-1">
							<div class="truncate font-semibold">{group.organization.name}</div>
							<div class="text-xs text-muted-foreground">Organization overview</div>
						</div>
						{#if organizationContext.active && organizationContext.organizationId === group.organization.id}
							<Check class="size-4 shrink-0" />
						{/if}
					</DropdownMenu.Item>
				{:else}
					<DropdownMenu.Label class="flex items-center gap-2 py-2">
						<Building2 class="size-5 shrink-0 text-muted-foreground" />
						<span class="truncate">{group.organization.name}</span>
					</DropdownMenu.Label>
				{/if}

				{#each group.projects as project (project.id)}
					{@render projectItem(project)}
				{/each}
				{#if group.projects.length === 0}
					<DropdownMenu.Item disabled class="pl-4">No projects yet</DropdownMenu.Item>
				{/if}
			</DropdownMenu.Group>
		{/each}

		{#if groups.ungroupedProjects.length > 0}
			<DropdownMenu.Separator />
			<DropdownMenu.Group>
				<DropdownMenu.Label class="text-muted-foreground">Other projects</DropdownMenu.Label>
				{#each groups.ungroupedProjects as project (project.id)}
					{@render projectItem(project)}
				{/each}
			</DropdownMenu.Group>
		{/if}

		{#if projectsState.projects.length === 0}
			<DropdownMenu.Item disabled>No projects yet</DropdownMenu.Item>
		{/if}

		<DropdownMenu.Separator />
		{#if !organizationContext.active && projectsState.canManageCurrentProject}
			<DropdownMenu.Item onclick={onEditProject} class="cursor-pointer">
				<Pencil class="size-4" />
				Edit Project
			</DropdownMenu.Item>
		{/if}
		<DropdownMenu.Item onclick={onAddProject} class="cursor-pointer">
			<Plus class="size-4" />
			Add Project
		</DropdownMenu.Item>
	</DropdownMenu.Content>
</DropdownMenu.Root>
