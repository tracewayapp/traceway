<script lang="ts">
	import { resolve } from '$app/paths';
	import * as Sheet from '$lib/components/ui/sheet';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import * as Tabs from '$lib/components/ui/tabs';
	import * as Select from '$lib/components/ui/select';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { ErrorAlert } from '$lib/components/ui/error-alert';
	import { Checkbox } from '$lib/components/ui/checkbox';
	import { DEFAULT_HEALTHCHECK_PATHS } from '$lib/utils/healthcheck';
	import { projectsState, type Project, type Framework } from '$lib/state/projects.svelte';
	import { Check, Trash2, CircleAlert } from '@lucide/svelte';
	import CopyButton from '$lib/components/traceway/copy-button.svelte';
	import { underlineTabTriggerClass, underlineTabListClass } from '$lib/utils/tabs';
	import { cn } from '$lib/utils';
	import FrameworkCombobox from './framework-combobox.svelte';
	import { toast } from 'svelte-sonner';
	import { goto } from '$app/navigation';

	interface Props {
		open: boolean;
		onOpenChange: (open: boolean) => void;
		project: Project | null;
		initialTab?: string;
	}

	let { open, onOpenChange, project, initialTab = 'project' }: Props = $props();

	const PROFILE_LABEL_KEY_REGEX = /^[a-zA-Z0-9._:-]+$/;

	let activeTab = $state('project');
	let projectName = $state('');
	let selectedFramework = $state<Framework>('gin');
	let dropHealthyHealthchecks = $state(true);
	let healthcheckPathsText = $state('');
	let showDefaultHealthcheckPaths = $state(false);
	let profileLabelsText = $state('');
	let aiFlaggedTermsText = $state('');
	let aiFlaggedLanguages = $state<string[]>(['en']);
	let loading = $state(false);

	// Must match contentflag.AvailableLanguages() on the backend.
	const FLAGGED_LANGUAGE_OPTIONS = [
		{ code: 'en', label: 'English' },
		{ code: 'de', label: 'German' },
		{ code: 'es', label: 'Spanish' },
		{ code: 'fr', label: 'French' },
		{ code: 'it', label: 'Italian' },
		{ code: 'pt', label: 'Portuguese' },
		{ code: 'sr', label: 'Serbian' }
	];

	const flaggedLanguagesLabel = $derived(
		aiFlaggedLanguages.length === 0
			? 'None (custom terms only)'
			: FLAGGED_LANGUAGE_OPTIONS.filter((o) => aiFlaggedLanguages.includes(o.code))
					.map((o) => o.label)
					.join(', ')
	);
	let error = $state('');
	let showDeleteConfirm = $state(false);
	let deleting = $state(false);
	let deleteConfirmName = $state('');

	$effect(() => {
		if (open && project) {
			activeTab = initialTab;
			projectName = project.name;
			selectedFramework = project.framework;
			dropHealthyHealthchecks = project.dropHealthyHealthchecks ?? true;
			healthcheckPathsText = (project.healthcheckPaths ?? []).join('\n');
			profileLabelsText = (project.profileLabelAllowlist ?? []).join('\n');
			aiFlaggedTermsText = (project.aiFlaggedTerms ?? []).join('\n');
			aiFlaggedLanguages = project.aiFlaggedLanguages ?? ['en'];
			error = '';
		}
	});

	$effect(() => {
		if (!showDeleteConfirm) {
			deleteConfirmName = '';
		}
	});

	let deleteConfirmMatches = $derived(!!project && deleteConfirmName === project.name);

	let subtitle = $derived(
		activeTab === 'healthchecks'
			? 'Control which healthcheck requests are stored.'
			: activeTab === 'profiles'
				? 'Choose which pprof sample labels are recorded and searchable.'
				: activeTab === 'ai'
					? 'Flag AI conversations that contain specific terms.'
					: activeTab === 'danger'
						? 'Irreversible and destructive actions.'
						: ''
	);

	let profileLabelKeys = $derived(
		profileLabelsText
			.split('\n')
			.map((p) => p.trim())
			.filter((p) => p.length > 0)
	);

	let profileLabelError = $derived.by(() => {
		const keys = profileLabelKeys;
		if (keys.length > 20) {
			return 'At most 20 profile label keys are allowed.';
		}
		for (const key of keys) {
			if (key.toLowerCase() === 'endpoint') continue;
			if (key.length > 100) {
				return 'Profile label keys must be at most 100 characters.';
			}
			if (!PROFILE_LABEL_KEY_REGEX.test(key)) {
				return `"${key}" may only contain letters, numbers, and . _ : -`;
			}
		}
		return '';
	});

	let aiFlaggedTerms = $derived(
		aiFlaggedTermsText
			.split('\n')
			.map((t) => t.trim().toLowerCase())
			.filter((t) => t.length > 0)
	);

	let aiFlaggedTermsError = $derived.by(() => {
		if (aiFlaggedTerms.length > 200) {
			return 'At most 200 flagged terms are allowed.';
		}
		for (const term of aiFlaggedTerms) {
			if (term.length > 100) {
				return 'Flagged terms must be at most 100 characters.';
			}
		}
		return '';
	});

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
			.map((p) => p.trim())
			.filter((p) => p.length > 0);

		const profileLabelAllowlist = profileLabelsText
			.split('\n')
			.map((p) => p.trim())
			.filter((p) => p.length > 0);

		try {
			await projectsState.updateProject(
				project.id,
				projectName.trim(),
				selectedFramework,
				dropHealthyHealthchecks,
				healthcheckPaths,
				profileLabelAllowlist,
				aiFlaggedTerms,
				aiFlaggedLanguages
			);
			toast.success('Successfully updated the Project', { position: 'top-center' });
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
			toast.success('Successfully deleted the Project');
			showDeleteConfirm = false;
			onOpenChange(false);
			goto(resolve('/'));
		} catch (err: any) {
			toast.error(err instanceof Error ? err.message : 'Failed to delete project');
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
	<Sheet.Content side="right" class="w-full overflow-y-auto sm:w-[680px] sm:max-w-[680px]">
		<Sheet.Header class="px-6 pb-0">
			<Sheet.Title>Edit Project</Sheet.Title>
		</Sheet.Header>

		<Tabs.Root
			value={activeTab}
			onValueChange={(v) => {
				if (v) activeTab = v;
			}}
		>
			<Tabs.List class={cn(underlineTabListClass, 'flex-nowrap overflow-x-auto px-6')}>
				<Tabs.Trigger value="project" class={underlineTabTriggerClass}>Project</Tabs.Trigger>
				<Tabs.Trigger value="healthchecks" class={underlineTabTriggerClass}
					>Healthchecks</Tabs.Trigger
				>
				<Tabs.Trigger value="profiles" class={underlineTabTriggerClass}>Profiles</Tabs.Trigger>
				<Tabs.Trigger value="ai" class={underlineTabTriggerClass}>AI</Tabs.Trigger>
				<Tabs.Trigger value="danger" class={underlineTabTriggerClass}>Danger Zone</Tabs.Trigger>
			</Tabs.List>
		</Tabs.Root>

		{#if subtitle}
			<Sheet.Description class="px-6">{subtitle}</Sheet.Description>
		{/if}

		{#if activeTab === 'danger'}
			<div class="space-y-3 px-6 pb-6">
				<p class="text-sm text-muted-foreground">
					Permanently delete this project along with all of its data, including transactions,
					exceptions, logs, metrics, and dashboards.
				</p>
				<Button
					type="button"
					variant="destructiveOutline"
					onclick={() => (showDeleteConfirm = true)}
				>
					<Trash2 class="mr-2 h-4 w-4" />
					Delete Project
				</Button>
			</div>
		{:else}
			<form onsubmit={handleSubmit} class="space-y-5 px-6 pb-6">
				<ErrorAlert {error} />

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
				{:else if activeTab === 'healthchecks'}
					<div class="flex items-start gap-2">
						<Checkbox
							checked={dropHealthyHealthchecks}
							onCheckedChange={(checked) => (dropHealthyHealthchecks = checked === true)}
							disabled={loading}
							class="mt-0.5"
							aria-label="Drop healthy healthcheck requests"
						/>
						<div class="space-y-1">
							<Label
								class="cursor-pointer"
								onclick={() => {
									if (!loading) dropHealthyHealthchecks = !dropHealthyHealthchecks;
								}}>Drop healthy healthcheck requests</Label
							>
							<p class="text-xs text-muted-foreground">
								Requests to common healthcheck endpoints (GET/HEAD) are only stored when they fail
								with status 400 or higher.
								<button
									type="button"
									class="underline hover:text-foreground"
									onclick={() => (showDefaultHealthcheckPaths = !showDefaultHealthcheckPaths)}
								>
									{showDefaultHealthcheckPaths ? 'Hide' : 'Show'} built-in paths
								</button>
							</p>
							{#if showDefaultHealthcheckPaths}
								<div class="flex flex-wrap gap-1 pt-1">
									{#each DEFAULT_HEALTHCHECK_PATHS as path, __index (__index)}
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
								placeholder={'/internal/probe\n/checks/*'}
								class="flex w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-xs transition-[color,box-shadow] outline-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-input/30"
							></textarea>
							<p class="text-xs text-muted-foreground">
								One path per line. Use a trailing * to match a prefix or a leading * to match a
								suffix.
							</p>
						</div>
					{/if}
				{:else if activeTab === 'profiles'}
					<div class="space-y-2">
						<Label>Built-in label</Label>
						<div class="flex flex-wrap gap-1">
							<code class="rounded bg-muted px-1.5 py-0.5 text-xs">endpoint</code>
						</div>
						<p class="text-xs text-muted-foreground">
							The <code class="rounded bg-muted px-1 py-0.5 text-[0.7rem]">endpoint</code> label is always
							recorded and searchable.
						</p>
					</div>

					<div class="space-y-2">
						<Label for="edit-profile-labels">Additional profile labels</Label>
						<textarea
							id="edit-profile-labels"
							bind:value={profileLabelsText}
							disabled={loading}
							rows="3"
							placeholder={'tenant\nregion'}
							class="flex w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-xs transition-[color,box-shadow] outline-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-input/30"
						></textarea>
						{#if profileLabelError}
							<p class="flex items-center gap-1 text-xs text-destructive">
								<CircleAlert class="h-3.5 w-3.5 shrink-0" />
								{profileLabelError}
							</p>
						{/if}
						<p class="text-xs text-muted-foreground">
							One label key per line. These pprof sample labels are recorded at ingest and become
							searchable on the flame-graph page. Added keys only apply to profiles ingested
							afterward.
						</p>
						<p class="text-xs text-muted-foreground">
							Avoid high-cardinality keys like <code
								class="rounded bg-muted px-1 py-0.5 text-[0.7rem]">user_id</code
							>
							or <code class="rounded bg-muted px-1 py-0.5 text-[0.7rem]">request_id</code>: every
							distinct value is stored as a separate sample, which bloats storage and slows queries.
						</p>
					</div>
				{:else if activeTab === 'ai'}
					<div class="space-y-2">
						<Label>Flagged term languages</Label>
						<Select.Root type="multiple" bind:value={aiFlaggedLanguages} disabled={loading}>
							<Select.Trigger class="w-full">
								<span class="truncate">{flaggedLanguagesLabel}</span>
							</Select.Trigger>
							<Select.Content>
								{#each FLAGGED_LANGUAGE_OPTIONS as option (option.code)}
									<Select.Item value={option.code} label={option.label}>
										{option.label}
									</Select.Item>
								{/each}
							</Select.Content>
						</Select.Root>
						<p class="text-xs text-muted-foreground">
							Built-in profanity lists to scan conversations with. Deselect all to rely on custom
							terms only. Matching is per whole word, so these packs only cover languages with
							space-separated words.
						</p>
					</div>

					<div class="space-y-2">
						<Label for="edit-ai-flagged-terms">Custom flagged terms</Label>
						<textarea
							id="edit-ai-flagged-terms"
							bind:value={aiFlaggedTermsText}
							disabled={loading}
							rows="4"
							placeholder={'competitor name\nrefund'}
							class="flex w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-xs transition-[color,box-shadow] outline-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-input/30"
						></textarea>
						{#if aiFlaggedTermsError}
							<p class="flex items-center gap-1 text-xs text-destructive">
								<CircleAlert class="h-3.5 w-3.5 shrink-0" />
								{aiFlaggedTermsError}
							</p>
						{/if}
						<p class="text-xs text-muted-foreground">
							One term per line. AI conversations containing these terms, in addition to the
							selected language packs, are marked as flagged and filterable on the Conversations
							page. Terms match whole words, case-insensitively, and only apply to calls ingested
							afterward.
						</p>
					</div>
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
				This project will be permanently deleted along with all of its data, including transactions,
				exceptions, logs, metrics, and dashboards.
			</AlertDialog.Description>
		</AlertDialog.Header>

		<div class="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2">
			<p class="text-sm">
				<span class="font-semibold text-destructive">Warning:</span>
				<span class="text-destructive/90">This action is not reversible. Please be certain.</span>
			</p>
		</div>

		<div class="space-y-2">
			<Label
				for="delete-confirm-name"
				class="text-sm leading-relaxed font-normal text-muted-foreground"
			>
				Enter the project name <span class="font-semibold text-foreground">{project?.name}</span
				><CopyButton
					bare
					text={() => project?.name ?? ''}
					title="Copy project name"
					iconClass="h-3.5 w-3.5"
					class="ml-1 inline-flex items-center p-0.5 align-middle hover:bg-accent"
				/> to continue:
			</Label>
			<Input
				id="delete-confirm-name"
				type="text"
				autocomplete="off"
				bind:value={deleteConfirmName}
				disabled={deleting}
			/>
			{#if deleteConfirmName.length > 0 && !deleteConfirmMatches}
				<p class="flex items-center gap-1 text-xs text-destructive">
					<CircleAlert class="h-3.5 w-3.5" />
					The project name does not match
				</p>
			{/if}
		</div>

		<AlertDialog.Footer class="sm:justify-between">
			<Button variant="outline" onclick={() => (showDeleteConfirm = false)} disabled={deleting}>
				Cancel
			</Button>
			<Button
				variant="destructive"
				onclick={handleDelete}
				disabled={deleting || !deleteConfirmMatches}
			>
				<Trash2 class="mr-2 h-4 w-4" />
				{deleting ? 'Deleting...' : 'Delete Project'}
			</Button>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
