<script lang="ts">
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { ErrorAlert } from '$lib/components/ui/error-alert';
	import * as Select from '$lib/components/ui/select';
	import TagsInput from '$lib/components/traceway/tags-input.svelte';
	import { Plus, Check } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';
	import { api } from '$lib/api';
	import type { AgentProfile } from '$lib/types/agent';

	interface Props {
		open: boolean;
		organizationId: number;
		profile: AgentProfile | null;
		onSaved: () => void;
	}

	let { open = $bindable(), organizationId, profile, onSaved }: Props = $props();

	const AGENTS = [{ value: 'claude-code', label: 'Claude Code' }];

	let name = $state('');
	let agent = $state('claude-code');
	let model = $state('');
	let provider = $state('');
	let baseUrl = $state('');
	let credential = $state('');
	let maxTurns = $state(60);
	let timeoutMinutes = $state(45);
	let budgetUsd = $state(5);
	let allowedTools = $state<string[]>([]);
	let isDefault = $state(false);
	let loading = $state(false);
	let error = $state('');

	const isEditing = $derived(profile !== null);

	function reset() {
		error = '';
		credential = '';
		name = profile?.name ?? '';
		agent = profile?.agent ?? 'claude-code';
		model = profile?.model ?? '';
		provider = profile?.provider ?? '';
		baseUrl = profile?.baseUrl ?? '';
		maxTurns = profile?.maxTurns ?? 60;
		timeoutMinutes = profile?.timeoutMinutes ?? 45;
		budgetUsd = profile?.budgetUsd ?? 5;
		allowedTools = [...(profile?.allowedTools ?? [])];
		isDefault = profile?.isDefault ?? false;
	}

	$effect(() => {
		if (open) reset();
	});

	async function submit() {
		loading = true;
		error = '';
		const body = {
			name,
			agent,
			model,
			provider,
			baseUrl,
			credential,
			maxTurns: Number(maxTurns),
			timeoutMinutes: Number(timeoutMinutes),
			budgetUsd: Number(budgetUsd),
			allowedTools,
			isDefault
		};
		try {
			if (isEditing && profile) {
				await api.put(`/organizations/${organizationId}/agent-profiles/${profile.id}`, body);
				toast.success('Successfully updated the Profile', { position: 'top-center' });
			} else {
				await api.post(`/organizations/${organizationId}/agent-profiles`, body);
				toast.success('Successfully created the Profile', { position: 'top-center' });
			}
			open = false;
			onSaved();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to save the profile';
		} finally {
			loading = false;
		}
	}
</script>

<AlertDialog.Root {open} onOpenChange={(value) => (open = value)}>
	<AlertDialog.Content class="max-h-[90vh] max-w-md overflow-y-auto">
		<AlertDialog.Header>
			<AlertDialog.Title>{isEditing ? 'Edit Profile' : 'New Profile'}</AlertDialog.Title>
			<AlertDialog.Description>
				Which coding agent runs an attempt, with which model and key, and under which limits.
			</AlertDialog.Description>
		</AlertDialog.Header>

		<form
			class="space-y-4"
			onsubmit={(e) => {
				e.preventDefault();
				submit();
			}}
		>
			<ErrorAlert {error} />
			<div class="space-y-2">
				<Label for="profile-name">Name</Label>
				<Input id="profile-name" bind:value={name} placeholder="e.g. Claude Opus" required />
			</div>
			<div class="space-y-2">
				<Label>Agent</Label>
				<Select.Root type="single" bind:value={agent}>
					<Select.Trigger class="w-full"
						>{AGENTS.find((a) => a.value === agent)?.label}</Select.Trigger
					>
					<Select.Content>
						{#each AGENTS as option (option.value)}
							<Select.Item value={option.value}>{option.label}</Select.Item>
						{/each}
					</Select.Content>
				</Select.Root>
			</div>
			<div class="space-y-2">
				<Label for="profile-model">Model</Label>
				<Input id="profile-model" bind:value={model} placeholder="claude-opus-5" />
			</div>
			<div class="space-y-2">
				<Label for="profile-credential">Provider API key</Label>
				<Input
					id="profile-credential"
					type="password"
					bind:value={credential}
					placeholder={profile?.hasCredential ? 'Secret set, leave blank to keep' : 'sk-ant-...'}
					required={!profile?.hasCredential}
				/>
				<p class="text-xs text-muted-foreground">
					Stored encrypted; handed to the agent process only, with egress limited to the provider.
				</p>
			</div>
			<div class="space-y-2">
				<Label for="profile-base-url">Base URL (optional)</Label>
				<Input
					id="profile-base-url"
					bind:value={baseUrl}
					placeholder="https://gateway.example.com"
				/>
			</div>
			<div class="grid grid-cols-3 gap-3">
				<div class="space-y-2">
					<Label for="profile-turns">Max turns</Label>
					<Input id="profile-turns" type="number" bind:value={maxTurns} min={0} max={500} />
				</div>
				<div class="space-y-2">
					<Label for="profile-timeout">Timeout (min)</Label>
					<Input
						id="profile-timeout"
						type="number"
						bind:value={timeoutMinutes}
						min={0}
						max={1440}
					/>
				</div>
				<div class="space-y-2">
					<Label for="profile-budget">Budget (USD)</Label>
					<Input id="profile-budget" type="number" step="0.5" bind:value={budgetUsd} min={0} />
				</div>
			</div>
			<div class="space-y-2">
				<Label>Allowed tools (optional)</Label>
				<TagsInput bind:tags={allowedTools} placeholder="Read, Edit, Bash..." />
			</div>
			<div class="flex items-center gap-2">
				<input
					id="profile-default"
					type="checkbox"
					bind:checked={isDefault}
					class="h-4 w-4 cursor-pointer"
				/>
				<Label for="profile-default" class="cursor-pointer"
					>Default profile for this organization</Label
				>
			</div>
		</form>

		<AlertDialog.Footer>
			<AlertDialog.Cancel disabled={loading}>Cancel</AlertDialog.Cancel>
			<Button variant={isEditing ? 'default' : 'success'} onclick={submit} disabled={loading}>
				{#if isEditing}
					<Check class="mr-2 h-4 w-4" />
					{loading ? 'Updating...' : 'Update Profile'}
				{:else}
					<Plus class="mr-2 h-4 w-4" />
					{loading ? 'Creating...' : 'New Profile'}
				{/if}
			</Button>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
