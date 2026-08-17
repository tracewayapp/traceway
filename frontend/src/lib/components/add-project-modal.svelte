<script lang="ts">
    import { untrack } from 'svelte';
    import * as Sheet from "$lib/components/ui/sheet";
    import { Button } from "$lib/components/ui/button";
    import { Input } from "$lib/components/ui/input";
    import { Label } from "$lib/components/ui/label";
    import * as Select from "$lib/components/ui/select";
    import { ErrorAlert } from "$lib/components/ui/error-alert";
    import { projectsState, getFrameworkLabel, type ProjectWithToken, type Framework } from '$lib/state/projects.svelte';
    import { authState } from '$lib/state/auth.svelte';
    import { Copy, Check, ExternalLink, Plus } from 'lucide-svelte';
    import FrameworkIcon from './framework-icon.svelte';
    import FrameworkCombobox from './framework-combobox.svelte';

    interface Props {
        open: boolean;
        onOpenChange: (open: boolean) => void;
        onProjectCreated: () => void;
    }

    let { open, onOpenChange, onProjectCreated }: Props = $props();

    let projectName = $state('');
    let selectedFramework = $state<Framework>('opentelemetry');
    let loading = $state(false);
    let error = $state('');
    let createdProject = $state<ProjectWithToken | null>(null);
    let copied = $state(false);
    let selectedOrgId = $state<number | null>(null);

    const writableOrgs = $derived(authState.organizations.filter(o => o.role !== 'readonly'));
    const selectedOrgName = $derived(writableOrgs.find(o => o.id === selectedOrgId)?.name ?? '');

    $effect(() => {
        if (open) {
            untrack(() => {
                const currentOrgId = projectsState.currentProject?.organizationId;
                selectedOrgId = writableOrgs.some(o => o.id === currentOrgId)
                    ? currentOrgId!
                    : (writableOrgs[0]?.id ?? null);
            });
        }
    });

    async function handleSubmit(e: Event) {
        e.preventDefault();

        loading = true;
        error = '';

        try {
            const project = await projectsState.createProject(projectName.trim(), selectedFramework, selectedOrgId ?? undefined);
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
        selectedFramework = 'opentelemetry';
        error = '';
        createdProject = null;
        selectedOrgId = null;
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
                    New Project
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
                    <Label>Organization</Label>
                    <div class="text-sm text-muted-foreground">{selectedOrgName}</div>
                </div>

                <div class="space-y-2">
                    <Label>Framework</Label>
                    <div class="flex items-center gap-2 text-sm text-muted-foreground">
                        <FrameworkIcon framework={selectedFramework} />
                        <span>{getFrameworkLabel(selectedFramework)}</span>
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
                <ErrorAlert {error} />

                <div class="space-y-2">
                    <Label>Organization</Label>
                    {#if writableOrgs.length > 1}
                        <Select.Root
                            type="single"
                            value={selectedOrgId !== null ? String(selectedOrgId) : undefined}
                            onValueChange={(val) => { if (val) selectedOrgId = Number(val); }}
                        >
                            <Select.Trigger class="w-full" disabled={loading}>
                                {selectedOrgName || 'Select organization'}
                            </Select.Trigger>
                            <Select.Content>
                                {#each writableOrgs as org (org.id)}
                                    <Select.Item value={String(org.id)}>{org.name}</Select.Item>
                                {/each}
                            </Select.Content>
                        </Select.Root>
                    {:else}
                        <div class="text-sm font-medium">{selectedOrgName}</div>
                    {/if}
                </div>

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
                    <FrameworkCombobox bind:value={selectedFramework} disabled={loading} />
                    <p class="text-xs text-muted-foreground">
                        Select your framework for tailored integration code
                    </p>
                </div>

                <div class="flex justify-end gap-2 pt-2">
                    <Button type="button" variant="outline" onclick={handleClose} disabled={loading}>
                        Cancel
                    </Button>
                    <Button type="submit" variant="success" disabled={loading}>
                        {#if loading}
                            Creating...
                        {:else}
                            <Plus class="mr-2 h-4 w-4" />
                            New Project
                        {/if}
                    </Button>
                </div>
            </form>
        {/if}
    </Sheet.Content>
</Sheet.Root>
