<script lang="ts">
	import { onMount } from 'svelte';
	import {
		Card,
		CardContent,
		CardDescription,
		CardHeader,
		CardTitle
	} from '$lib/components/ui/card';
	import { Button } from '$lib/components/ui/button';
	import { Badge } from '$lib/components/ui/badge';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { Pencil, Trash2, Plus, Layers } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';
	import {
		oncallState,
		type Schedule,
		type ScheduleOverride
	} from '$lib/state/oncall.svelte';
	import { formatDateTime } from '$lib/utils/formatters';
	import ScheduleDialog from './schedule-dialog.svelte';
	import ScheduleTimeline from './schedule-timeline.svelte';
	import LayerEditor from './layer-editor.svelte';
	import OverrideDialog from './override-dialog.svelte';

	interface Props {
		organizationId: number;
		scheduleId: number;
		canManage: boolean;
		onDeleted: () => void;
		onChanged: () => void;
	}

	let { organizationId, scheduleId, canManage, onDeleted, onChanged }: Props = $props();

	let schedule = $state<Schedule | null>(null);
	let overrides = $state<ScheduleOverride[]>([]);
	let loading = $state(true);
	let error = $state('');
	let editDialogOpen = $state(false);
	let overrideDialogOpen = $state(false);
	let showDeleteDialog = $state(false);
	let showLayerEditor = $state(false);
	let overrideToDelete = $state<ScheduleOverride | null>(null);
	let timelineRefreshKey = $state(0);

	const team = $derived(oncallState.teams.find((t) => t.id === schedule?.teamId));

	function userLabel(userId: number): string {
		const member = team?.members.find((m) => m.userId === userId);
		return member ? member.name || member.email : `User #${userId}`;
	}

	async function loadDetail() {
		loading = true;
		error = '';
		try {
			const res = await oncallState.getSchedule(organizationId, scheduleId);
			schedule = res.schedule;
			overrides = res.overrides || [];
		} catch (e: unknown) {
			error = e instanceof Error ? e.message : 'Failed to load schedule';
		} finally {
			loading = false;
		}
	}

	async function handleDeleteSchedule() {
		try {
			await oncallState.deleteSchedule(organizationId, scheduleId);
			toast.success('Successfully deleted the Schedule', { position: 'top-center' });
			showDeleteDialog = false;
			onDeleted();
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to delete schedule', {
				position: 'top-center'
			});
		}
	}

	async function handleDeleteOverride() {
		if (!overrideToDelete) return;
		try {
			await oncallState.deleteOverride(organizationId, scheduleId, overrideToDelete.id);
			toast.success('Successfully deleted the Override', { position: 'top-center' });
			overrides = overrides.filter((o) => o.id !== overrideToDelete!.id);
			timelineRefreshKey += 1;
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to delete override', {
				position: 'top-center'
			});
		} finally {
			overrideToDelete = null;
		}
	}

	onMount(() => {
		loadDetail();
	});
</script>

{#if loading}
	<div class="flex justify-center py-12"><LoadingCircle size="xlg" /></div>
{:else if error}
	<div class="rounded-md border py-8 text-center text-destructive">{error}</div>
{:else if schedule}
	<Card>
		<CardHeader class="flex flex-row items-start justify-between gap-4 space-y-0">
			<div class="min-w-0 space-y-1.5">
				<CardTitle class="flex flex-wrap items-center gap-2">
					{schedule.name}
					{#if team}
						<Badge variant="outline">{team.name}</Badge>
					{/if}
					<Badge variant="secondary">{schedule.timezone}</Badge>
				</CardTitle>
				{#if schedule.description}
					<CardDescription>{schedule.description}</CardDescription>
				{/if}
			</div>
			{#if canManage}
				<div class="flex shrink-0 gap-1">
					<Button
						variant="outline"
						size="sm"
						onclick={() => (showLayerEditor = !showLayerEditor)}
					>
						<Layers class="mr-1 h-4 w-4" /> Edit Layers
					</Button>
					<Button variant="ghost" size="icon" title="Edit" onclick={() => (editDialogOpen = true)}>
						<Pencil class="h-4 w-4" />
					</Button>
					<Button
						variant="ghost"
						size="icon"
						title="Delete"
						onclick={() => (showDeleteDialog = true)}
					>
						<Trash2 class="h-4 w-4" />
					</Button>
				</div>
			{/if}
		</CardHeader>
		<CardContent class="space-y-6">
			{#if showLayerEditor && canManage}
				<LayerEditor
					{organizationId}
					{schedule}
					onSaved={() => {
						showLayerEditor = false;
						timelineRefreshKey += 1;
						loadDetail();
						onChanged();
					}}
					onCancel={() => (showLayerEditor = false)}
				/>
			{/if}

			{#key timelineRefreshKey}
				<ScheduleTimeline {organizationId} {scheduleId} {schedule} />
			{/key}

			<div class="space-y-2">
				<div class="flex items-center justify-between">
					<h3 class="text-sm font-medium">Overrides (next 30 days)</h3>
					<Button size="sm" variant="outline" onclick={() => (overrideDialogOpen = true)}>
						<Plus class="mr-1 h-4 w-4" /> Add Override
					</Button>
				</div>
				{#if overrides.length === 0}
					<p class="text-sm text-muted-foreground">No upcoming overrides.</p>
				{:else}
					<div class="space-y-1">
						{#each overrides as override (override.id)}
							<div class="flex items-center justify-between gap-2 rounded-md border px-3 py-2">
								<div class="min-w-0 text-sm">
									<span class="font-medium">{userLabel(override.userId)}</span>
									<span class="text-muted-foreground">
										{formatDateTime(override.startAt, { format: 'short' })} — {formatDateTime(
											override.endAt,
											{ format: 'short' }
										)}
									</span>
								</div>
								<Button
									variant="ghost"
									size="icon"
									title="Delete"
									onclick={() => (overrideToDelete = override)}
								>
									<Trash2 class="h-4 w-4" />
								</Button>
							</div>
						{/each}
					</div>
				{/if}
			</div>
		</CardContent>
	</Card>

	<ScheduleDialog
		bind:open={editDialogOpen}
		{organizationId}
		{schedule}
		onSaved={() => {
			editDialogOpen = false;
			loadDetail();
			onChanged();
		}}
	/>

	<OverrideDialog
		bind:open={overrideDialogOpen}
		{organizationId}
		{schedule}
		onSaved={() => {
			overrideDialogOpen = false;
			timelineRefreshKey += 1;
			loadDetail();
		}}
	/>

	<AlertDialog.Root bind:open={showDeleteDialog}>
		<AlertDialog.Content>
			<AlertDialog.Header>
				<AlertDialog.Title>Delete Schedule</AlertDialog.Title>
				<AlertDialog.Description>
					Are you sure you want to delete "{schedule.name}"? This action cannot be undone.
				</AlertDialog.Description>
			</AlertDialog.Header>
			<AlertDialog.Footer>
				<AlertDialog.Cancel>Cancel</AlertDialog.Cancel>
				<AlertDialog.Action variant="destructive" onclick={handleDeleteSchedule}>
					<Trash2 class="mr-2 h-4 w-4" />
					Delete Schedule
				</AlertDialog.Action>
			</AlertDialog.Footer>
		</AlertDialog.Content>
	</AlertDialog.Root>

	<AlertDialog.Root
		open={overrideToDelete !== null}
		onOpenChange={(open) => {
			if (!open) overrideToDelete = null;
		}}
	>
		<AlertDialog.Content>
			<AlertDialog.Header>
				<AlertDialog.Title>Delete Override</AlertDialog.Title>
				<AlertDialog.Description>
					Are you sure you want to delete the override for {overrideToDelete
						? userLabel(overrideToDelete.userId)
						: ''}?
				</AlertDialog.Description>
			</AlertDialog.Header>
			<AlertDialog.Footer>
				<AlertDialog.Cancel>Cancel</AlertDialog.Cancel>
				<AlertDialog.Action variant="destructive" onclick={handleDeleteOverride}>
					<Trash2 class="mr-2 h-4 w-4" />
					Delete Override
				</AlertDialog.Action>
			</AlertDialog.Footer>
		</AlertDialog.Content>
	</AlertDialog.Root>
{/if}
