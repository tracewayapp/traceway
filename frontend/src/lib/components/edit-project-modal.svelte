<script lang="ts">
    import * as Sheet from "$lib/components/ui/sheet";
    import * as AlertDialog from "$lib/components/ui/alert-dialog";
    import * as Tabs from "$lib/components/ui/tabs";
    import { Button } from "$lib/components/ui/button";
    import { Input } from "$lib/components/ui/input";
    import { Label } from "$lib/components/ui/label";
    import { Checkbox } from "$lib/components/ui/checkbox";
    import { DEFAULT_HEALTHCHECK_PATHS } from '$lib/utils/healthcheck';
    import { projectsState, type Project, type Framework } from '$lib/state/projects.svelte';
    import { Check, Trash2, Copy, CircleAlert } from 'lucide-svelte';
    import FrameworkCombobox from './framework-combobox.svelte';
    import { toast } from 'svelte-sonner';
    import { goto } from '$app/navigation';

    interface Props {
        open: boolean;
        onOpenChange: (open: boolean) => void;
        project: Project | null;
    }

    let { open, onOpenChange, project }: Props = $props();

    const tabTriggerClass = "-mb-px rounded-none border-b-2 border-transparent bg-transparent px-0 pb-2.5 pt-0 text-sm font-medium text-muted-foreground shadow-none data-[state=active]:border-primary data-[state=active]:bg-transparent data-[state=active]:text-foreground data-[state=active]:shadow-none";

    let activeTab = $state('project');
    let projectName = $state('');
    let selectedFramework = $state<Framework>('gin');
    let dropHealthyHealthchecks = $state(true);
    let healthcheckPathsText = $state('');
    let showDefaultHealthcheckPaths = $state(false);
    let loading = $state(false);
    let error = $state('');
    let showDeleteConfirm = $state(false);
    let deleting = $state(false);
    let deleteConfirmName = $state('');
    let nameCopied = $state(false);

    $effect(() => {
        if (open && project) {
            activeTab = 'project';
            projectName = project.name;
            selectedFramework = project.framework;
            dropHealthyHealthchecks = project.dropHealthyHealthchecks ?? true;
            healthcheckPathsText = (project.healthcheckPaths ?? []).join('\n');
            error = '';
        }
    });

    $effect(() => {
        if (!showDeleteConfirm) {
            deleteConfirmName = '';
            nameCopied = false;
        }
    });

    async function copyProjectName() {
        if (!project) return;
        await navigator.clipboard.writeText(project.name);
        nameCopied = true;
        setTimeout(() => nameCopied = false, 2000);
    }

    let deleteConfirmMatches = $derived(
        !!project && deleteConfirmName === project.name
    );

    let subtitle = $derived(
        activeTab === 'healthchecks' ? 'Control which healthcheck requests are stored.'
        : activeTab === 'danger' ? 'Irreversible and destructive actions.'
        : ''
    );

    async function handleSubmit(e: Event) {
        e.preventDefault();
        if (!projectName.trim()) {
            error = 'Project name is required';
            return;
        }
        if (!project) return;

        loading = true;
        error = '';

        const healthcheckPaths = healthcheckPathsText
            .split('\n')
            .map(p => p.trim())
            .filter(p => p.length > 0);

        try {
            await projectsState.updateProject(project.id, projectName.trim(), selectedFramework, dropHealthyHealthchecks, healthcheckPaths);
            toast.success('Successfully updated the project', { position: 'top-center' });
            onOpenChange(false);
        } catch (err: any) {
            error = err instanceof Error ? err.message : 'Failed to update project';
        } finally {
            loading = false;
        }
    }

    async function handleDelete() {
        if (!project || !deleteConfirmMatches) return;

        deleting = true;
        try {
            await projectsState.deleteProject(project.id, project.name);
            toast.success('Successfully deleted the project', { position: 'top-center' });
            showDeleteConfirm = false;
            onOpenChange(false);
            goto('/');
        } catch (err: any) {
            toast.error(err instanceof Error ? err.message : 'Failed to delete project', { position: 'top-center' });
        } finally {
            deleting = false;
        }
    }

    function handleClose() {
        error = '';
        showDeleteConfirm = false;
        onOpenChange(false);
    }
</script>

<Sheet.Root {open} onOpenChange={handleClose}>
    <Sheet.Content side="right" class="w-full overflow-y-auto sm:w-[540px]">
        <Sheet.Header class="px-6 pb-0">
            <Sheet.Title>Edit Project</Sheet.Title>
        </Sheet.Header>

        <Tabs.Root value={activeTab} onValueChange={(v) => { if (v) activeTab = v; }}>
            <Tabs.List class="h-auto w-full justify-start gap-4 rounded-none border-b bg-transparent p-0 pl-6 pt-0">
                <Tabs.Trigger value="project" class={tabTriggerClass}>Project</Tabs.Trigger>
                <Tabs.Trigger value="healthchecks" class={tabTriggerClass}>Healthchecks</Tabs.Trigger>
                <Tabs.Trigger value="danger" class={tabTriggerClass}>Danger Zone</Tabs.Trigger>
            </Tabs.List>
        </Tabs.Root>

        {#if subtitle}
            <Sheet.Description class="px-6">{subtitle}</Sheet.Description>
        {/if}

        {#if activeTab === 'danger'}
            <div class="px-6 pb-6 space-y-3">
                <p class="text-sm text-muted-foreground">
                    Permanently delete this project along with all of its data, including transactions, exceptions, logs, metrics, and dashboards.
                </p>
                <Button
                    type="button"
                    variant="destructiveOutline"
                    onclick={() => showDeleteConfirm = true}
                >
                    <Trash2 class="mr-2 h-4 w-4" />
                    Delete Project
                </Button>
            </div>
        {:else}
            <form onsubmit={handleSubmit} class="px-6 pb-6 space-y-5">
                {#if error}
                    <div class="rounded-md bg-destructive/10 border border-destructive/20 p-3">
                        <p class="text-sm text-destructive">{error}</p>
                    </div>
                {/if}

                {#if activeTab === 'project'}
                    <div class="space-y-2">
                        <Label for="edit-project-name">Project Name</Label>
                        <Input
                            id="edit-project-name"
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
                        <Label for="edit-framework">Framework</Label>
                        <FrameworkCombobox bind:value={selectedFramework} disabled={loading} />
                        <p class="text-xs text-muted-foreground">
                            Select your framework for tailored integration code
                        </p>
                    </div>
                {:else}
                    <div class="flex items-start gap-2">
                        <Checkbox
                            checked={dropHealthyHealthchecks}
                            onCheckedChange={(checked) => dropHealthyHealthchecks = checked === true}
                            disabled={loading}
                            class="mt-0.5"
                            aria-label="Drop healthy healthcheck requests"
                        />
                        <div class="space-y-1">
                            <Label class="cursor-pointer" onclick={() => { if (!loading) dropHealthyHealthchecks = !dropHealthyHealthchecks; }}>Drop healthy healthcheck requests</Label>
                            <p class="text-xs text-muted-foreground">
                                Requests to common healthcheck endpoints (GET/HEAD) are only stored when they fail with status 400 or higher.
                                <button
                                    type="button"
                                    class="underline hover:text-foreground"
                                    onclick={() => showDefaultHealthcheckPaths = !showDefaultHealthcheckPaths}
                                >
                                    {showDefaultHealthcheckPaths ? 'Hide' : 'Show'} built-in paths
                                </button>
                            </p>
                            {#if showDefaultHealthcheckPaths}
                                <div class="flex flex-wrap gap-1 pt-1">
                                    {#each DEFAULT_HEALTHCHECK_PATHS as path}
                                        <code class="rounded bg-muted px-1.5 py-0.5 text-xs">{path}</code>
                                    {/each}
                                </div>
                            {/if}
                        </div>
                    </div>

                    {#if dropHealthyHealthchecks}
                        <div class="space-y-2">
                            <Label for="edit-healthcheck-paths">Additional healthcheck paths</Label>
                            <textarea
                                id="edit-healthcheck-paths"
                                bind:value={healthcheckPathsText}
                                disabled={loading}
                                rows="3"
                                placeholder={"/internal/probe\n/checks/*"}
                                class="border-input bg-background dark:bg-input/30 placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-ring/50 flex w-full rounded-md border px-3 py-2 text-sm shadow-xs transition-[color,box-shadow] outline-none focus-visible:ring-[3px] disabled:cursor-not-allowed disabled:opacity-50"
                            ></textarea>
                            <p class="text-xs text-muted-foreground">
                                One path per line. Use a trailing * to match a prefix or a leading * to match a suffix.
                            </p>
                        </div>
                    {/if}
                {/if}

                <div class="flex justify-end gap-2 pt-2">
                    <Button type="button" variant="outline" onclick={handleClose} disabled={loading}>
                        Cancel
                    </Button>
                    <Button type="submit" disabled={loading}>
                        {#if loading}
                            Updating...
                        {:else}
                            <Check class="mr-2 h-4 w-4" />
                            Update Project
                        {/if}
                    </Button>
                </div>
            </form>
        {/if}
    </Sheet.Content>
</Sheet.Root>

<AlertDialog.Root bind:open={showDeleteConfirm}>
    <AlertDialog.Content interactOutsideBehavior="close">
        <AlertDialog.Header>
            <AlertDialog.Title>Delete Project</AlertDialog.Title>
            <AlertDialog.Description>
                This project will be permanently deleted along with all of its data, including transactions, exceptions, logs, metrics, and dashboards.
            </AlertDialog.Description>
        </AlertDialog.Header>

        <div class="rounded-md bg-destructive/10 border border-destructive/30 px-3 py-2">
            <p class="text-sm">
                <span class="font-semibold text-destructive">Warning:</span>
                <span class="text-destructive/90">This action is not reversible. Please be certain.</span>
            </p>
        </div>

        <div class="space-y-2">
            <Label for="delete-confirm-name" class="text-sm font-normal text-muted-foreground leading-relaxed">
                Enter the project name <span class="font-semibold text-foreground">{project?.name}</span><button
                    type="button"
                    onclick={copyProjectName}
                    class="inline-flex items-center align-middle ml-1 rounded p-0.5 text-muted-foreground hover:bg-accent hover:text-foreground"
                    title="Copy project name"
                    aria-label="Copy project name"
                >
                    {#if nameCopied}
                        <Check class="h-3.5 w-3.5" />
                    {:else}
                        <Copy class="h-3.5 w-3.5" />
                    {/if}
                </button> to continue:
            </Label>
            <Input
                id="delete-confirm-name"
                type="text"
                autocomplete="off"
                bind:value={deleteConfirmName}
                disabled={deleting}
            />
            {#if deleteConfirmName.length > 0 && !deleteConfirmMatches}
                <p class="text-xs text-destructive flex items-center gap-1">
                    <CircleAlert class="h-3.5 w-3.5" />
                    The project name does not match
                </p>
            {/if}
        </div>

        <AlertDialog.Footer class="sm:justify-between">
            <Button variant="outline" onclick={() => showDeleteConfirm = false} disabled={deleting}>
                Cancel
            </Button>
            <Button variant="destructive" onclick={handleDelete} disabled={deleting || !deleteConfirmMatches}>
                {deleting ? 'Deleting...' : 'Delete Project'}
            </Button>
        </AlertDialog.Footer>
    </AlertDialog.Content>
</AlertDialog.Root>
