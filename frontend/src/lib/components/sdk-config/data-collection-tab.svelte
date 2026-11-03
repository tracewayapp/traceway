<script lang="ts">
	import SdkVersionWarning from './sdk-version-warning.svelte';
	import { onMount } from 'svelte';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Button } from '$lib/components/ui/button';
	import { Switch } from '$lib/components/ui/switch';
	import { ErrorAlert } from '$lib/components/ui/error-alert';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { ArrowDown, ArrowUp, Eye, Pencil, Plus, Trash2, X } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';
	import { api } from '$lib/api';
	import { getErrorMessage, getErrorStatus } from '$lib/utils/errors';
	import { projectsState } from '$lib/state/projects.svelte';
	import SdkConfigRuleDialog from './sdk-config-rule-dialog.svelte';
	import SdkConfigPreviewDialog from './sdk-config-preview-dialog.svelte';
	import SdkCaptureDialog from './sdk-capture-dialog.svelte';
	import {
		summarizeConditions,
		summarizeSettings,
		type SdkCapture,
		type SdkConfigRule
	} from './sdk-config';

	let rules = $state<SdkConfigRule[]>([]);
	let captures = $state<SdkCapture[]>([]);
	let heartbeatSeconds = $state(300);
	let loading = $state(true);
	let busy = $state(false);
	let error = $state('');
	let ruleDialogOpen = $state(false);
	let editingRule = $state<SdkConfigRule | null>(null);
	let previewOpen = $state(false);
	let captureOpen = $state(false);
	let deletingRule = $state<SdkConfigRule | null>(null);

	const canWrite = $derived(projectsState.canWriteCurrentProject);
	const activeCaptures = $derived(captures.filter((c) => c.active));

	async function load() {
		const projectId = projectsState.currentProjectId;
		if (!projectId) return;
		error = '';
		try {
			const [ruleList, captureList] = await Promise.all([
				api.get('/sdk-config/rules', { projectId }) as Promise<{
					data: SdkConfigRule[];
					heartbeatIntervalSeconds: number;
				}>,
				api.get('/sdk-config/captures', { projectId }) as Promise<{ data: SdkCapture[] }>
			]);
			rules = ruleList.data;
			heartbeatSeconds = ruleList.heartbeatIntervalSeconds;
			captures = captureList.data;
		} catch (e) {
			error = getErrorMessage(e) || 'Failed to load data collection rules';
		} finally {
			loading = false;
		}
	}

	onMount(load);

	async function mutate(action: () => Promise<unknown>, success?: string) {
		busy = true;
		error = '';
		try {
			await action();
			if (success) toast.success(success, { position: 'top-center' });
			await load();
		} catch (e) {
			if (getErrorStatus(e) !== 403) error = getErrorMessage(e) || 'The change failed';
		} finally {
			busy = false;
		}
	}

	function toggle(rule: SdkConfigRule, enabled: boolean) {
		const projectId = projectsState.currentProjectId ?? undefined;
		mutate(() =>
			api.put(
				`/sdk-config/rules/${rule.id}`,
				{ name: rule.name, enabled, conditions: rule.conditions, settings: rule.settings },
				{ projectId }
			)
		);
	}

	function move(index: number, delta: number) {
		const ids = rules.map((r) => r.id);
		const [moved] = ids.splice(index, 1);
		ids.splice(index + delta, 0, moved);
		mutate(() =>
			api.put(
				'/sdk-config/rules/reorder',
				{ ids },
				{ projectId: projectsState.currentProjectId ?? undefined }
			)
		);
	}

	function confirmDelete() {
		const rule = deletingRule;
		if (!rule) return;
		deletingRule = null;
		mutate(
			() =>
				api.delete(`/sdk-config/rules/${rule.id}`, {
					projectId: projectsState.currentProjectId ?? undefined
				}),
			'Successfully deleted the Rule'
		);
	}

	function cancelCapture(capture: SdkCapture) {
		mutate(
			() =>
				api.delete(`/sdk-config/captures/${capture.id}`, {
					projectId: projectsState.currentProjectId ?? undefined
				}),
			'Successfully cancelled the Capture'
		);
	}

	function remaining(expiresAt: string): string {
		const minutes = Math.max(0, Math.round((Date.parse(expiresAt) - Date.now()) / 60_000));
		return minutes >= 120 ? `${Math.round(minutes / 60)} h left` : `${minutes} min left`;
	}
</script>

<div class="space-y-6 px-6 pb-6">
	<SdkVersionWarning />
	<p class="text-sm text-muted-foreground">
		The browser SDK asks Traceway for these settings over OpAMP. Clients pick up a change within
		about
		{Math.max(1, Math.round(heartbeatSeconds / 60))} min. Returning to a tab checks again once its poll
		interval has elapsed.
	</p>
	<ErrorAlert {error} />

	{#if loading}
		<div class="flex justify-center py-6"><LoadingCircle /></div>
	{:else}
		<section class="space-y-3">
			<div class="flex items-center justify-between">
				<h3 class="text-sm font-medium">Rules</h3>
				<div class="flex gap-2">
					<Button variant="outline" size="sm" onclick={() => (previewOpen = true)}>
						<Eye class="mr-2 h-4 w-4" />
						Preview
					</Button>
					{#if canWrite}
						<Button
							variant="success"
							size="sm"
							onclick={() => {
								editingRule = null;
								ruleDialogOpen = true;
							}}
							disabled={busy}
						>
							<Plus class="mr-2 h-4 w-4" />
							New Rule
						</Button>
					{/if}
				</div>
			</div>

			{#if rules.length === 0}
				<p class="rounded-md border border-dashed p-4 text-sm text-muted-foreground">
					No rules: errors are sent with duplicate and burst protection. The preceding 30 seconds of
					breadcrumbs and replay are uploaded on an error. Lightweight session metadata and web
					vitals stay on; healthy-session sampling is off. Explicit SDK options can override these
					defaults.
				</p>
			{:else}
				<ul class="divide-y rounded-md border">
					{#each rules as rule, index (rule.id)}
						<li class="flex items-start gap-3 p-3">
							<Switch
								checked={rule.enabled}
								onCheckedChange={(checked) => toggle(rule, checked)}
								disabled={!canWrite || busy}
								aria-label="Enable rule"
								class="mt-1"
							/>
							<div class="min-w-0 flex-1">
								<div class="text-sm font-medium" class:text-muted-foreground={!rule.enabled}>
									{rule.name}
								</div>
								<div class="text-xs text-muted-foreground">
									{summarizeConditions(rule.conditions)} → {summarizeSettings(rule.settings)}
								</div>
							</div>
							{#if canWrite}
								<div class="flex shrink-0 items-center">
									<Button
										variant="ghost"
										size="icon"
										aria-label="Move up"
										onclick={() => move(index, -1)}
										disabled={busy || index === 0}
									>
										<ArrowUp class="h-4 w-4" />
									</Button>
									<Button
										variant="ghost"
										size="icon"
										aria-label="Move down"
										onclick={() => move(index, 1)}
										disabled={busy || index === rules.length - 1}
									>
										<ArrowDown class="h-4 w-4" />
									</Button>
									<Button
										variant="ghost"
										size="icon"
										aria-label="Edit rule"
										onclick={() => {
											editingRule = rule;
											ruleDialogOpen = true;
										}}
										disabled={busy}
									>
										<Pencil class="h-4 w-4" />
									</Button>
									<Button
										variant="ghost"
										size="icon"
										aria-label="Delete rule"
										onclick={() => (deletingRule = rule)}
										disabled={busy}
									>
										<Trash2 class="h-4 w-4" />
									</Button>
								</div>
							{/if}
						</li>
					{/each}
				</ul>
			{/if}
		</section>

		<section class="space-y-3">
			<div class="flex items-center justify-between">
				<h3 class="text-sm font-medium">Active captures</h3>
				{#if canWrite}
					<Button variant="success" size="sm" onclick={() => (captureOpen = true)} disabled={busy}>
						<Plus class="mr-2 h-4 w-4" />
						New Capture
					</Button>
				{/if}
			</div>
			{#if activeCaptures.length === 0}
				<p class="text-sm text-muted-foreground">
					None. A capture makes one device, user or session upload what it buffered and send
					everything for a while; start one here or from a session or issue.
				</p>
			{:else}
				<ul class="divide-y rounded-md border">
					{#each activeCaptures as capture (capture.id)}
						<li class="flex items-center justify-between gap-3 p-3 text-sm">
							<span class="min-w-0 truncate font-mono text-xs">{capture.label}</span>
							<span class="flex shrink-0 items-center gap-2 text-xs text-muted-foreground">
								{remaining(capture.expiresAt)}
								{#if canWrite}
									<Button
										variant="ghost"
										size="icon"
										aria-label="Cancel capture"
										onclick={() => cancelCapture(capture)}
										disabled={busy}
									>
										<X class="h-4 w-4" />
									</Button>
								{/if}
							</span>
						</li>
					{/each}
				</ul>
			{/if}
		</section>
	{/if}
</div>

<SdkConfigRuleDialog bind:open={ruleDialogOpen} rule={editingRule} onSaved={load} />
<SdkConfigPreviewDialog bind:open={previewOpen} />
<SdkCaptureDialog bind:open={captureOpen} onCreated={load} />

<AlertDialog.Root
	open={deletingRule !== null}
	onOpenChange={(open) => {
		if (!open) deletingRule = null;
	}}
>
	<AlertDialog.Content class="sm:max-w-md">
		<AlertDialog.Header>
			<AlertDialog.Title>Delete Rule</AlertDialog.Title>
			<AlertDialog.Description>
				Clients matching "{deletingRule?.name}" go back to the earlier rules and SDK defaults at
				their next poll.
			</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel>Cancel</AlertDialog.Cancel>
			<Button variant="destructive" onclick={confirmDelete}>
				<Trash2 class="mr-2 h-4 w-4" />
				Delete Rule
			</Button>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
