<script lang="ts">
    import * as Sheet from "$lib/components/ui/sheet";
    import * as Select from "$lib/components/ui/select";
    import { Button } from "$lib/components/ui/button";
    import { Input } from "$lib/components/ui/input";
    import { Label } from "$lib/components/ui/label";
    import { projectsState, type ProjectWithToken, type Framework } from '$lib/state/projects.svelte';
    import { Copy, Check, ExternalLink } from 'lucide-svelte';
    import FrameworkIcon from './framework-icon.svelte';

    interface Props {
        open: boolean;
        onOpenChange: (open: boolean) => void;
        onProjectCreated: () => void;
    }

    let { open, onOpenChange, onProjectCreated }: Props = $props();

    let projectName = $state('');
    let selectedFramework = $state<Framework>('gin');
    let loading = $state(false);
    let error = $state('');
    let createdProject = $state<ProjectWithToken | null>(null);
    let copied = $state(false);

    const frameworks = [
        { value: 'gin', label: 'Gin', description: 'Fast HTTP web framework', group: 'Go' },
        { value: 'fiber', label: 'Fiber', description: 'Express-inspired framework', group: 'Go' },
        { value: 'chi', label: 'Chi', description: 'Lightweight router', group: 'Go' },
        { value: 'fasthttp', label: 'FastHTTP', description: 'High-performance HTTP', group: 'Go' },
        { value: 'stdlib', label: 'Standard Library', description: 'net/http package', group: 'Go' },
        { value: 'custom', label: 'Custom', description: 'Other / manual setup', group: 'Go' },
        { value: 'react', label: 'React', description: 'Frontend UI library', group: 'JavaScript' },
        { value: 'svelte', label: 'Svelte', description: 'Frontend compiler framework', group: 'JavaScript' },
        { value: 'vuejs', label: 'Vue.js', description: 'Progressive frontend framework', group: 'JavaScript' },
        { value: 'nextjs', label: 'Next.js', description: 'React full-stack framework', group: 'JavaScript' },
        { value: 'nestjs', label: 'NestJS', description: 'Node.js server framework', group: 'JavaScript' },
        { value: 'express', label: 'Express', description: 'Minimal Node.js framework', group: 'JavaScript' },
        { value: 'remix', label: 'Remix', description: 'React full-stack framework', group: 'JavaScript' },
    ] as const;

    const selectedFrameworkLabel = $derived(
        frameworks.find(f => f.value === selectedFramework)?.label ?? 'Select framework'
    );

    async function handleSubmit(e: Event) {
        e.preventDefault();
        if (!projectName.trim()) {
            error = 'Project name is required';
            return;
        }

        loading = true;
        error = '';

        try {
            const project = await projectsState.createProject(projectName.trim(), selectedFramework);
            createdProject = project;
        } catch (err) {
            error = err instanceof Error ? err.message : 'Failed to create project';
        } finally {
            loading = false;
        }
    }

    async function copyToken() {
        if (createdProject?.token) {
            await navigator.clipboard.writeText(createdProject.token);
            copied = true;
            setTimeout(() => copied = false, 2000);
        }
    }

    function handleClose() {
        projectName = '';
        selectedFramework = 'gin';
        error = '';
        createdProject = null;
        onOpenChange(false);
    }

    function handleGoToConnection() {
        if (createdProject) {
            projectsState.selectProject(createdProject.id);
        }
        handleClose();
        onProjectCreated();
    }
</script>

<Sheet.Root {open} onOpenChange={handleClose}>
    <Sheet.Content side="right" class="w-[400px] sm:w-[540px]">
        <Sheet.Header>
            <Sheet.Title>
                {#if createdProject}
                    Project Created!
                {:else}
                    Create New Project
                {/if}
            </Sheet.Title>
            <Sheet.Description>
                {#if createdProject}
                    Your project has been created. Save the token below - you'll need it to connect your application.
                {:else}
                    Add a new project to start tracking errors and metrics.
                {/if}
            </Sheet.Description>
        </Sheet.Header>

        {#if createdProject}
            <div class="px-6 py-6 space-y-6">
                <div class="space-y-2">
                    <Label>Project Name</Label>
                    <div class="text-lg font-medium">{createdProject.name}</div>
                </div>

                <div class="space-y-2">
                    <Label>Framework</Label>
                    <div class="flex items-center gap-2 text-sm text-muted-foreground">
                        <FrameworkIcon framework={selectedFramework} />
                        <span>{selectedFrameworkLabel}</span>
                    </div>
                </div>

                <div class="space-y-2">
                    <Label>Project Token</Label>
                    <div class="flex gap-2">
                        <Input
                            type="text"
                            value={createdProject.token}
                            readonly
                            class="font-mono text-sm"
                        />
                        <Button variant="outline" size="icon" onclick={copyToken}>
                            {#if copied}
                                <Check class="h-4 w-4 text-green-500" />
                            {:else}
                                <Copy class="h-4 w-4" />
                            {/if}
                        </Button>
                    </div>
                    <p class="text-sm text-muted-foreground">
                        This token will only be shown once. Make sure to save it securely.
                    </p>
                </div>
            </div>

            <div class="flex justify-end gap-2 px-6 pb-6">
                <Button variant="outline" onclick={handleClose}>
                    Close
                </Button>
                <Button onclick={handleGoToConnection}>
                    <ExternalLink class="mr-2 h-4 w-4" />
                    Go to Connection
                </Button>
            </div>
        {:else}
            <form onsubmit={handleSubmit} class="px-6 py-6 space-y-5">
                <div class="space-y-2">
                    <Label for="project-name">Project Name</Label>
                    <Input
                        id="project-name"
                        type="text"
                        placeholder="My Application"
                        bind:value={projectName}
                        disabled={loading}
                    />
                    <p class="text-xs text-muted-foreground">
                        A unique name for your project (letters, numbers, spaces, hyphens)
                    </p>
                </div>

                <div class="space-y-2">
                    <Label for="framework">Framework</Label>
                    <Select.Root type="single" bind:value={selectedFramework}>
                        <Select.Trigger class="w-full">
                            <div class="flex items-center gap-2">
                                <FrameworkIcon framework={selectedFramework} />
                                <span>{selectedFrameworkLabel}</span>
                            </div>
                        </Select.Trigger>
                        <Select.Content>
                            <Select.Group>
                                <Select.GroupHeading class="px-2 py-1.5 text-xs font-semibold text-muted-foreground">Go</Select.GroupHeading>
                                {#each frameworks.filter(f => f.group === 'Go') as framework}
                                    <Select.Item value={framework.value}>
                                        {#snippet children({ selected })}
                                            <div class="flex items-center gap-2">
                                                <FrameworkIcon framework={framework.value} />
                                                <div class="flex flex-col">
                                                    <span class="font-medium">{framework.label}</span>
                                                    <span class="text-xs text-muted-foreground">{framework.description}</span>
                                                </div>
                                            </div>
                                            {#if selected}
                                                <Check class="absolute end-2 size-4" />
                                            {/if}
                                        {/snippet}
                                    </Select.Item>
                                {/each}
                            </Select.Group>
                            <Select.Group>
                                <Select.GroupHeading class="px-2 py-1.5 text-xs font-semibold text-muted-foreground">JavaScript</Select.GroupHeading>
                                {#each frameworks.filter(f => f.group === 'JavaScript') as framework}
                                    <Select.Item value={framework.value}>
                                        {#snippet children({ selected })}
                                            <div class="flex items-center gap-2">
                                                <FrameworkIcon framework={framework.value} />
                                                <div class="flex flex-col">
                                                    <span class="font-medium">{framework.label}</span>
                                                    <span class="text-xs text-muted-foreground">{framework.description}</span>
                                                </div>
                                            </div>
                                            {#if selected}
                                                <Check class="absolute end-2 size-4" />
                                            {/if}
                                        {/snippet}
                                    </Select.Item>
                                {/each}
                            </Select.Group>
                        </Select.Content>
                    </Select.Root>
                    <p class="text-xs text-muted-foreground">
                        Select your framework for tailored integration code
                    </p>
                </div>

                {#if error}
                    <div class="rounded-md bg-destructive/10 border border-destructive/20 p-3">
                        <p class="text-sm text-destructive">{error}</p>
                    </div>
                {/if}

                <div class="flex justify-end gap-2 pt-2">
                    <Button type="button" variant="outline" onclick={handleClose} disabled={loading}>
                        Cancel
                    </Button>
                    <Button type="submit" disabled={loading || !projectName.trim()}>
                        {#if loading}
                            Creating...
                        {:else}
                            Create Project
                        {/if}
                    </Button>
                </div>
            </form>
        {/if}
    </Sheet.Content>
</Sheet.Root>
