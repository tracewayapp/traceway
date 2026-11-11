<script lang="ts">
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { CodeEditor } from '$lib/components/ui/code-editor';
	import { ErrorAlert } from '$lib/components/ui/error-alert';
	import { Eye } from '@lucide/svelte';
	import { api } from '$lib/api';
	import { getErrorMessage } from '$lib/utils/errors';
	import { projectsState } from '$lib/state/projects.svelte';

	interface Props {
		open: boolean;
	}

	let { open = $bindable() }: Props = $props();

	interface PreviewResult {
		settings: Record<string, unknown>;
		configHash: string;
		matchedRules: Array<{ id: number; name: string }>;
		matchedCaptures: string[];
		heartbeatIntervalSeconds: number;
	}

	let attributesText = $state('service.version=2.4.0\nbrowser.name=Chrome\nuser.id=');
	let instanceUid = $state('');
	let result = $state<PreviewResult | null>(null);
	let resultJson = $state('');
	let loading = $state(false);
	let dialogError = $state('');

	$effect(() => {
		if (open) {
			result = null;
			dialogError = '';
		}
	});

	function parseAttributes(): Record<string, string> {
		const attributes: Record<string, string> = {};
		for (const line of attributesText.split('\n')) {
			const index = line.indexOf('=');
			if (index <= 0) continue;
			const key = line.slice(0, index).trim();
			const value = line.slice(index + 1).trim();
			if (key && value) attributes[key] = value;
		}
		return attributes;
	}

	async function preview() {
		const projectId = projectsState.currentProjectId;
		if (!projectId) return;
		loading = true;
		dialogError = '';
		try {
			result = (await api.post(
				'/sdk-config/preview',
				{ attributes: parseAttributes(), instanceUid: instanceUid.trim() || undefined },
				{ projectId }
			)) as PreviewResult;
			resultJson = JSON.stringify(result.settings, null, 2);
		} catch (e) {
			dialogError = getErrorMessage(e) || 'Failed to preview the settings';
		} finally {
			loading = false;
		}
	}
</script>

<AlertDialog.Root bind:open>
	<AlertDialog.Content class="max-h-[90vh] overflow-y-auto sm:max-w-xl">
		<AlertDialog.Header>
			<AlertDialog.Title>Preview Settings</AlertDialog.Title>
			<AlertDialog.Description>
				See which rules a client with these attributes matches and the settings it receives.
			</AlertDialog.Description>
		</AlertDialog.Header>

		<div class="space-y-4">
			<ErrorAlert error={dialogError} />
			<div class="space-y-2">
				<Label for="sdk-preview-attributes">Attributes, one key=value per line</Label>
				<textarea
					id="sdk-preview-attributes"
					class="min-h-28 w-full rounded-md border bg-transparent px-3 py-2 font-mono text-sm"
					bind:value={attributesText}
				></textarea>
			</div>
			<div class="space-y-2">
				<Label for="sdk-preview-instance">Instance id (optional)</Label>
				<Input
					id="sdk-preview-instance"
					placeholder="from getInstanceId()"
					bind:value={instanceUid}
				/>
			</div>

			{#if result}
				<div class="space-y-2 text-sm">
					<div>
						<span class="text-muted-foreground">Matched rules:</span>
						{result.matchedRules.length > 0
							? result.matchedRules.map((r) => r.name).join(', ')
							: 'none (the SDK defaults apply)'}
					</div>
					{#if result.matchedCaptures.length > 0}
						<div>
							<span class="text-muted-foreground">Active captures:</span>
							{result.matchedCaptures.length}
						</div>
					{/if}
					<div class="text-muted-foreground">
						Polls every {Math.round(result.heartbeatIntervalSeconds / 60)} min, and when the tab regains
						focus.
					</div>
					<CodeEditor value={resultJson} readOnly minHeight="160px" />
				</div>
			{/if}
		</div>

		<AlertDialog.Footer>
			<AlertDialog.Cancel>Close</AlertDialog.Cancel>
			<Button onclick={preview} disabled={loading}>
				<Eye class="mr-2 h-4 w-4" />
				{loading ? 'Previewing...' : 'Preview'}
			</Button>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
