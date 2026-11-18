<script lang="ts">
	import SdkVersionWarning from './sdk-version-warning.svelte';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import * as Select from '$lib/components/ui/select';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Checkbox } from '$lib/components/ui/checkbox';
	import { ErrorAlert } from '$lib/components/ui/error-alert';
	import { Plus } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';
	import { api } from '$lib/api';
	import { getErrorMessage, getErrorStatus } from '$lib/utils/errors';
	import { projectsState } from '$lib/state/projects.svelte';

	type TargetKind = 'instance' | 'user' | 'session' | 'attribute';

	interface Props {
		open: boolean;
		kind?: TargetKind;
		value?: string;
		onCreated?: () => void;
	}

	let { open = $bindable(), kind = 'instance', value = '', onCreated }: Props = $props();

	const kindLabels: Record<TargetKind, string> = {
		instance: 'Device (instance id)',
		user: 'User (user.id)',
		session: 'Session',
		attribute: 'Attribute'
	};
	const durations = [
		{ value: '15', label: '15 minutes' },
		{ value: '60', label: '1 hour' },
		{ value: '240', label: '4 hours' },
		{ value: '1440', label: '24 hours' }
	];
	const lookbacks = [
		{ value: '0', label: 'Nothing earlier' },
		{ value: '60', label: 'Last minute' },
		{ value: '600', label: 'Last 10 minutes' },
		{ value: '3600', label: 'Last hour' }
	];

	let targetKind = $state<TargetKind>('instance');
	let targetValue = $state('');
	let attributeKey = $state('');
	let durationMinutes = $state('60');
	let lookbackSeconds = $state('600');
	let replay = $state(true);
	let saving = $state(false);
	let dialogError = $state('');

	$effect(() => {
		if (open) {
			targetKind = kind;
			targetValue = value;
			attributeKey = '';
			durationMinutes = '60';
			lookbackSeconds = '600';
			replay = true;
			dialogError = '';
		}
	});

	async function create() {
		const projectId = projectsState.currentProjectId;
		if (!projectId) return;
		saving = true;
		dialogError = '';
		try {
			await api.post(
				'/sdk-config/captures',
				{
					target: { kind: targetKind, key: attributeKey, value: targetValue },
					durationMinutes: Number(durationMinutes),
					lookbackSeconds: Number(lookbackSeconds),
					replay
				},
				{ projectId }
			);
			toast.success('Successfully created the Capture', { position: 'top-center' });
			open = false;
			onCreated?.();
		} catch (e) {
			if (getErrorStatus(e) !== 403) {
				dialogError = getErrorMessage(e) || 'Failed to start the capture';
			}
		} finally {
			saving = false;
		}
	}
</script>

<AlertDialog.Root bind:open>
	<AlertDialog.Content class="sm:max-w-md" interactOutsideBehavior={saving ? 'ignore' : 'close'}>
		<AlertDialog.Header>
			<AlertDialog.Title>New Capture</AlertDialog.Title>
			<AlertDialog.Description>
				Matching browsers upload the selected lookback and stream allowed signals until expiry.
				Captures last at most 24 hours, with at most 20 active per Project. They take effect on the
				next poll; returning to a tab polls only after the polling interval has elapsed.
			</AlertDialog.Description>
		</AlertDialog.Header>
		<SdkVersionWarning />
		<p
			class="text-xs text-muted-foreground"
			title="Each browser session can upload up to 50 MiB and 10,000 diagnostic events, including replay events, across all captures. Developers may lower these limits. Errors, lifecycle metadata and web vitals are separate. Limits persist across page reloads when session storage is available."
		>
			Captures respect a shared per-session diagnostic budget: 50 MiB / 10,000 events maximum.
		</p>

		<form
			class="space-y-4"
			onsubmit={(e) => {
				e.preventDefault();
				create();
			}}
		>
			<ErrorAlert error={dialogError} />
			<div class="space-y-2">
				<Label>Target</Label>
				<Select.Root type="single" bind:value={targetKind} disabled={saving}>
					<Select.Trigger class="w-full">{kindLabels[targetKind]}</Select.Trigger>
					<Select.Content>
						{#each Object.entries(kindLabels) as [k, label] (k)}
							<Select.Item value={k}>{label}</Select.Item>
						{/each}
					</Select.Content>
				</Select.Root>
			</div>
			{#if targetKind === 'attribute'}
				<div class="space-y-2">
					<Label for="sdk-capture-key">Attribute</Label>
					<Input
						id="sdk-capture-key"
						placeholder="tenant.id"
						bind:value={attributeKey}
						disabled={saving}
					/>
				</div>
			{/if}
			<div class="space-y-2">
				<Label for="sdk-capture-value">Value</Label>
				<Input id="sdk-capture-value" bind:value={targetValue} disabled={saving} />
			</div>
			<div class="grid grid-cols-2 gap-3">
				<div class="space-y-2">
					<Label>For</Label>
					<Select.Root type="single" bind:value={durationMinutes} disabled={saving}>
						<Select.Trigger class="w-full"
							>{durations.find((d) => d.value === durationMinutes)?.label}</Select.Trigger
						>
						<Select.Content>
							{#each durations as d (d.value)}
								<Select.Item value={d.value}>{d.label}</Select.Item>
							{/each}
						</Select.Content>
					</Select.Root>
				</div>
				<div class="space-y-2">
					<Label>Include</Label>
					<Select.Root type="single" bind:value={lookbackSeconds} disabled={saving}>
						<Select.Trigger class="w-full"
							>{lookbacks.find((l) => l.value === lookbackSeconds)?.label}</Select.Trigger
						>
						<Select.Content>
							{#each lookbacks as l (l.value)}
								<Select.Item value={l.value}>{l.label}</Select.Item>
							{/each}
						</Select.Content>
					</Select.Root>
				</div>
			</div>
			<div class="flex items-center gap-2">
				<Checkbox
					checked={replay}
					onCheckedChange={(checked) => (replay = checked === true)}
					disabled={saving}
					aria-label="Record session replay"
				/>
				<Label
					class="cursor-pointer"
					onclick={() => {
						if (!saving) replay = !replay;
					}}>Record session replay</Label
				>
			</div>
		</form>

		<AlertDialog.Footer>
			<AlertDialog.Cancel disabled={saving}>Cancel</AlertDialog.Cancel>
			<Button variant="success" onclick={create} disabled={saving}>
				<Plus class="mr-2 h-4 w-4" />
				{saving ? 'Starting...' : 'New Capture'}
			</Button>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
