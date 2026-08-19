<script lang="ts">
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Button } from '$lib/components/ui/button';
	import { Clock } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';
	import { api } from '$lib/api';
	import { projectsState } from '$lib/state/projects.svelte';

	interface Props {
		open: boolean;
		ruleId: number | null;
		onSaved: () => void;
	}

	let { open = $bindable(), ruleId, onSaved }: Props = $props();

	let loading = $state(false);

	const presets = [
		{ label: '30 minutes', minutes: 30 },
		{ label: '1 hour', minutes: 60 },
		{ label: '4 hours', minutes: 240 },
		{ label: '8 hours', minutes: 480 },
		{ label: '24 hours', minutes: 1440 }
	];

	async function snooze(durationMinutes: number) {
		if (!ruleId) return;
		loading = true;

		try {
			await api.post(
				`/notification-rules/${ruleId}/snooze`,
				{ durationMinutes },
				{ projectId: projectsState.currentProjectId ?? undefined }
			);
			toast.success('Rule snoozed');
			onSaved();
		} catch {
			toast.error('Failed to snooze rule');
		} finally {
			loading = false;
		}
	}

	async function clearSnooze() {
		if (!ruleId) return;
		loading = true;

		try {
			await api.post(
				`/notification-rules/${ruleId}/snooze`,
				{ durationMinutes: 0 },
				{ projectId: projectsState.currentProjectId ?? undefined }
			);
			toast.success('Snooze cleared');
			onSaved();
		} catch {
			toast.error('Failed to clear snooze');
		} finally {
			loading = false;
		}
	}

	function handleOpenChange(isOpen: boolean) {
		open = isOpen;
	}
</script>

<AlertDialog.Root {open} onOpenChange={handleOpenChange}>
	<AlertDialog.Content class="max-w-sm">
		<AlertDialog.Header>
			<AlertDialog.Title>Snooze Rule</AlertDialog.Title>
			<AlertDialog.Description>
				Temporarily silence this rule for a set duration
			</AlertDialog.Description>
		</AlertDialog.Header>

		<div class="grid gap-2">
			{#each presets as preset, __index (__index)}
				<Button
					variant="outline"
					class="w-full justify-start"
					disabled={loading}
					onclick={() => snooze(preset.minutes)}
				>
					<Clock class="mr-2 h-4 w-4" />
					{preset.label}
				</Button>
			{/each}
			<Button
				variant="ghost"
				class="w-full justify-start text-muted-foreground"
				disabled={loading}
				onclick={clearSnooze}
			>
				Clear snooze
			</Button>
		</div>

		<AlertDialog.Footer>
			<AlertDialog.Cancel disabled={loading}>Cancel</AlertDialog.Cancel>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
