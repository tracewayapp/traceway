<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { getFrameworkLabel, type Project } from '$lib/state/projects.svelte';
	import FrameworkIcon from '$lib/components/framework-icon.svelte';

	let {
		projects,
		initialProjectIds,
		waitingText
	}: {
		projects: Project[];
		initialProjectIds: Set<string>;
		waitingText: string;
	} = $props();
</script>

<div class="rounded-md border bg-card">
	<div class="border-b px-4 py-3">
		<h3 class="font-semibold">Projects</h3>
	</div>
	<div class="p-4">
		{#if projects.length === 0}
			<div class="flex items-center gap-3 text-sm text-muted-foreground">
				<span class="relative flex h-2.5 w-2.5">
					<span
						class="absolute inline-flex h-full w-full animate-ping rounded-full bg-primary opacity-60"
					></span>
					<span class="relative inline-flex h-2.5 w-2.5 rounded-full bg-primary"></span>
				</span>
				{waitingText}
			</div>
		{:else}
			<ul class="space-y-2">
				{#each projects as project (project.id)}
					{@const isNew = !initialProjectIds.has(project.id)}
					<li
						class="flex items-center gap-3 rounded-md border px-3 py-2 {isNew ? '' : 'opacity-60'}"
					>
						<FrameworkIcon framework={project.framework} class="size-5 shrink-0" />
						<div class="min-w-0 flex-1">
							<div class="truncate text-sm font-medium">{project.name}</div>
							<div class="text-xs text-muted-foreground">
								{getFrameworkLabel(project.framework)}
							</div>
						</div>
						{#if isNew}
							<Badge
								variant="outline"
								class="border-green-600/40 bg-green-500/10 text-green-600 dark:text-green-400"
							>
								Created
							</Badge>
						{:else}
							<Badge variant="outline" class="text-muted-foreground">Existing</Badge>
						{/if}
					</li>
				{/each}
			</ul>
		{/if}
	</div>
</div>
