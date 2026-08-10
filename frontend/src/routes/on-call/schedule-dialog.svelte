<script lang="ts">
	import { untrack } from 'svelte';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Select from '$lib/components/ui/select';
	import { ErrorAlert } from '$lib/components/ui/error-alert';
	import { Plus, Check } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';
	import { authState } from '$lib/state/auth.svelte';
	import { oncallState, type Schedule } from '$lib/state/oncall.svelte';

	interface Props {
		open: boolean;
		organizationId: number;
		schedule: Schedule | null;
		onSaved: (scheduleId?: number) => void;
	}

	let { open = $bindable(), organizationId, schedule, onSaved }: Props = $props();

	let name = $state('');
	let description = $state('');
	let teamId = $state<number | null>(null);
	let timezone = $state('');
	let loading = $state(false);
	let error = $state('');

	const isEditing = $derived(schedule !== null);
	const teams = $derived(oncallState.teams);
	const orgTimezone = $derived(
		authState.organizations.find((o) => o.id === organizationId)?.timezone ?? 'UTC'
	);
	const timezones: string[] =
		typeof Intl.supportedValuesOf === 'function' ? Intl.supportedValuesOf('timeZone') : [];

	// Reset only on the open transition, with teams untracked, so teams
	// resolving mid-edit can't wipe fields the user has already typed.
	let wasOpen = false;
	$effect(() => {
		if (open && !wasOpen) {
			if (schedule) {
				name = schedule.name;
				description = schedule.description;
				teamId = schedule.teamId;
				timezone = schedule.timezone;
			} else {
				name = '';
				description = '';
				teamId = untrack(() => teams[0]?.id ?? null);
				timezone = '';
			}
			error = '';
		}
		wasOpen = open;
	});

	async function handleSubmit() {
		loading = true;
		error = '';
		try {
			const body = {
				teamId: teamId ?? 0,
				name,
				description,
				timezone,
				definition: schedule?.definition ?? { schemaVersion: 1, layers: [] }
			};
			if (isEditing && schedule) {
				await oncallState.updateSchedule(organizationId, schedule.id, body);
				toast.success('Successfully updated the Schedule', { position: 'top-center' });
				onSaved(schedule.id);
			} else {
				const created = await oncallState.createSchedule(organizationId, body);
				toast.success('Successfully created the Schedule', { position: 'top-center' });
				onSaved(created?.id);
			}
		} catch (e: unknown) {
			error = e instanceof Error ? e.message : 'Failed to save schedule';
		} finally {
			loading = false;
		}
	}
</script>

<AlertDialog.Root {open} onOpenChange={(isOpen) => (open = isOpen)}>
	<AlertDialog.Content class="max-h-[90vh] max-w-md overflow-y-auto">
		<AlertDialog.Header>
			<AlertDialog.Title>{isEditing ? 'Edit Schedule' : 'New Schedule'}</AlertDialog.Title>
			<AlertDialog.Description>
				{isEditing
					? 'Update the schedule details'
					: 'Create an on-call schedule for a team'}
			</AlertDialog.Description>
		</AlertDialog.Header>

		<form
			onsubmit={(e) => {
				e.preventDefault();
				handleSubmit();
			}}
			class="space-y-4"
		>
			<ErrorAlert {error} />

			<div class="space-y-2">
				<Label for="schedule-name">Name</Label>
				<Input id="schedule-name" bind:value={name} placeholder="e.g. Primary" />
			</div>

			<div class="space-y-2">
				<Label for="schedule-description">Description</Label>
				<Input
					id="schedule-description"
					bind:value={description}
					placeholder="What this schedule covers"
				/>
			</div>

			<div class="space-y-2">
				<Label>Team</Label>
				<Select.Root
					type="single"
					value={teamId !== null ? String(teamId) : undefined}
					onValueChange={(val) => {
						if (val) teamId = Number(val);
					}}
				>
					<Select.Trigger class="w-full">
						{teams.find((t) => t.id === teamId)?.name ?? 'Select team'}
					</Select.Trigger>
					<Select.Content>
						{#each teams as team (team.id)}
							<Select.Item value={String(team.id)}>{team.name}</Select.Item>
						{/each}
					</Select.Content>
				</Select.Root>
			</div>

			<div class="space-y-2">
				<Label for="schedule-timezone">Timezone</Label>
				{#if timezones.length > 0}
					<Select.Root
						type="single"
						value={timezone || undefined}
						onValueChange={(val) => {
							if (val) timezone = val;
						}}
					>
						<Select.Trigger class="w-full">
							{timezone || `Organization default (${orgTimezone})`}
						</Select.Trigger>
						<Select.Content class="max-h-64 overflow-y-auto">
							{#each timezones as tz (tz)}
								<Select.Item value={tz}>{tz}</Select.Item>
							{/each}
						</Select.Content>
					</Select.Root>
				{:else}
					<Input id="schedule-timezone" bind:value={timezone} placeholder={orgTimezone} />
				{/if}
				<p class="text-xs text-muted-foreground">
					Leave empty to use the organization timezone ({orgTimezone}).
				</p>
			</div>
		</form>

		<AlertDialog.Footer>
			<AlertDialog.Cancel disabled={loading}>Cancel</AlertDialog.Cancel>
			<Button variant={isEditing ? 'default' : 'success'} onclick={handleSubmit} disabled={loading}>
				{#if isEditing}
					<Check class="mr-2 h-4 w-4" />
					{loading ? 'Updating...' : 'Update Schedule'}
				{:else}
					<Plus class="mr-2 h-4 w-4" />
					{loading ? 'Creating...' : 'New Schedule'}
				{/if}
			</Button>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
