<script lang="ts">
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Select from '$lib/components/ui/select';
	import { ErrorAlert } from '$lib/components/ui/error-alert';
	import { Plus } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';
	import {
		oncallState,
		getCurrentUserFromToken,
		type Schedule
	} from '$lib/state/oncall.svelte';

	interface Props {
		open: boolean;
		organizationId: number;
		schedule: Schedule;
		onSaved: () => void;
	}

	let { open = $bindable(), organizationId, schedule, onSaved }: Props = $props();

	let userId = $state<number | null>(null);
	let startAt = $state('');
	let endAt = $state('');
	let loading = $state(false);
	let error = $state('');

	const browserTz = Intl.DateTimeFormat().resolvedOptions().timeZone;

	const candidates = $derived.by(() => {
		const team = oncallState.teams.find((t) => t.id === schedule.teamId);
		const list = (team?.members ?? []).map((m) => ({
			userId: m.userId,
			label: m.name || m.email
		}));
		const currentUser = getCurrentUserFromToken();
		if (currentUser && !list.some((c) => c.userId === currentUser.userId)) {
			list.push({ userId: currentUser.userId, label: `You (${currentUser.email})` });
		}
		return list;
	});

	function toLocalInputValue(date: Date): string {
		const pad = (n: number) => String(n).padStart(2, '0');
		return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
	}

	$effect(() => {
		if (open) {
			const now = new Date();
			const inEightHours = new Date(now.getTime() + 8 * 3600 * 1000);
			startAt = toLocalInputValue(now);
			endAt = toLocalInputValue(inEightHours);
			userId = getCurrentUserFromToken()?.userId ?? candidates[0]?.userId ?? null;
			error = '';
		}
	});

	async function handleSubmit() {
		loading = true;
		error = '';
		try {
			await oncallState.createOverride(organizationId, schedule.id, {
				userId: userId ?? 0,
				startAt: startAt ? new Date(startAt).toISOString() : '',
				endAt: endAt ? new Date(endAt).toISOString() : ''
			});
			toast.success('Successfully created the Override', { position: 'top-center' });
			onSaved();
		} catch (e: unknown) {
			error = e instanceof Error ? e.message : 'Failed to create override';
		} finally {
			loading = false;
		}
	}
</script>

<AlertDialog.Root {open} onOpenChange={(isOpen) => (open = isOpen)}>
	<AlertDialog.Content class="max-w-md">
		<AlertDialog.Header>
			<AlertDialog.Title>Add Override</AlertDialog.Title>
			<AlertDialog.Description>
				Temporarily put someone on call for "{schedule.name}". Overrides can cover up to 30 days.
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
				<Label>User</Label>
				<Select.Root
					type="single"
					value={userId !== null ? String(userId) : undefined}
					onValueChange={(val) => {
						if (val) userId = Number(val);
					}}
				>
					<Select.Trigger class="w-full">
						{candidates.find((c) => c.userId === userId)?.label ?? 'Select user'}
					</Select.Trigger>
					<Select.Content>
						{#each candidates as candidate (candidate.userId)}
							<Select.Item value={String(candidate.userId)}>{candidate.label}</Select.Item>
						{/each}
					</Select.Content>
				</Select.Root>
			</div>

			<div class="space-y-2">
				<Label for="override-start">Start</Label>
				<Input id="override-start" type="datetime-local" bind:value={startAt} />
			</div>

			<div class="space-y-2">
				<Label for="override-end">End</Label>
				<Input id="override-end" type="datetime-local" bind:value={endAt} />
			</div>

			<p class="text-xs text-muted-foreground">
				{#if schedule.timezone && schedule.timezone !== browserTz}
					Times are in your local timezone ({browserTz}). This schedule uses {schedule.timezone}.
				{:else}
					Times are in your local timezone ({browserTz}).
				{/if}
			</p>
		</form>

		<AlertDialog.Footer>
			<AlertDialog.Cancel disabled={loading}>Cancel</AlertDialog.Cancel>
			<Button variant="success" onclick={handleSubmit} disabled={loading}>
				<Plus class="mr-2 h-4 w-4" />
				{loading ? 'Creating...' : 'New Override'}
			</Button>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
