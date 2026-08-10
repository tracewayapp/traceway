<script lang="ts">
	import { onMount } from 'svelte';
	import { DateTime } from 'luxon';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Select from '$lib/components/ui/select';
	import { ErrorAlert } from '$lib/components/ui/error-alert';
	import {
		Plus,
		Check,
		Trash2,
		ChevronUp,
		ChevronDown,
		X
	} from '@lucide/svelte';
	import { toast } from 'svelte-sonner';
	import {
		oncallState,
		type Schedule,
		type ScheduleLayer,
		type ScheduleRestriction,
		type OrgMember
	} from '$lib/state/oncall.svelte';

	interface Props {
		organizationId: number;
		schedule: Schedule;
		onSaved: () => void;
		onCancel: () => void;
	}

	let { organizationId, schedule, onSaved, onCancel }: Props = $props();

	let layers = $state<ScheduleLayer[]>([]);
	let members = $state<OrgMember[]>([]);
	let loading = $state(false);
	let error = $state('');

	const dayOptions = [
		{ value: 1, label: 'Monday' },
		{ value: 2, label: 'Tuesday' },
		{ value: 3, label: 'Wednesday' },
		{ value: 4, label: 'Thursday' },
		{ value: 5, label: 'Friday' },
		{ value: 6, label: 'Saturday' },
		{ value: 7, label: 'Sunday' }
	];

	const rotationOptions = [
		{ value: 'daily', label: 'Daily' },
		{ value: 'weekly', label: 'Weekly' },
		{ value: 'custom', label: 'Custom interval' }
	];

	function dayLabel(day: number | undefined): string {
		return dayOptions.find((d) => d.value === day)?.label ?? 'Select day';
	}

	function memberLabel(userId: number): string {
		const member = members.find((m) => m.id === userId);
		return member ? member.name || member.email : `User #${userId}`;
	}

	// Today in the schedule's timezone, so the default rotation phase isn't off
	// by a day for users whose local date differs from the schedule-local date.
	function todayISODate(): string {
		return (
			DateTime.now().setZone(schedule.timezone).toISODate() ??
			new Date().toISOString().slice(0, 10)
		);
	}

	function addLayer() {
		layers = [
			...layers,
			{
				name: `Layer ${layers.length + 1}`,
				rotationType: 'weekly',
				handoffTime: '09:00',
				handoffDay: 1,
				rotationStart: todayISODate(),
				userIds: [],
				restrictions: []
			}
		];
	}

	function removeLayer(index: number) {
		layers = layers.filter((_, i) => i !== index);
	}

	function moveLayer(index: number, direction: -1 | 1) {
		const target = index + direction;
		if (target < 0 || target >= layers.length) return;
		const next = [...layers];
		[next[index], next[target]] = [next[target], next[index]];
		layers = next;
	}

	function addLayerMember(layer: ScheduleLayer, userId: number) {
		if (!layer.userIds.includes(userId)) {
			layer.userIds = [...layer.userIds, userId];
		}
	}

	function removeLayerMember(layer: ScheduleLayer, index: number) {
		layer.userIds = layer.userIds.filter((_, i) => i !== index);
	}

	function moveLayerMember(layer: ScheduleLayer, index: number, direction: -1 | 1) {
		const target = index + direction;
		if (target < 0 || target >= layer.userIds.length) return;
		const next = [...layer.userIds];
		[next[index], next[target]] = [next[target], next[index]];
		layer.userIds = next;
	}

	function addRestriction(layer: ScheduleLayer) {
		layer.restrictions = [
			...layer.restrictions,
			{ type: 'daily', startTime: '09:00', endTime: '17:00' }
		];
	}

	function removeRestriction(layer: ScheduleLayer, index: number) {
		layer.restrictions = layer.restrictions.filter((_, i) => i !== index);
	}

	function setRestrictionType(restriction: ScheduleRestriction, type: 'daily' | 'weekly') {
		restriction.type = type;
		if (type === 'weekly') {
			restriction.startDay = restriction.startDay ?? 1;
			restriction.endDay = restriction.endDay ?? 5;
		} else {
			delete restriction.startDay;
			delete restriction.endDay;
		}
	}

	async function handleSave() {
		loading = true;
		error = '';
		try {
			const cleaned = layers.map((layer) => {
				const out: ScheduleLayer = { ...layer, userIds: [...layer.userIds] };
				if (layer.rotationType !== 'weekly') delete out.handoffDay;
				if (layer.rotationType !== 'custom') delete out.intervalDays;
				return out;
			});
			await oncallState.updateSchedule(organizationId, schedule.id, {
				teamId: schedule.teamId,
				name: schedule.name,
				description: schedule.description,
				timezone: schedule.timezone,
				definition: { schemaVersion: 1, layers: cleaned }
			});
			toast.success('Successfully updated the Schedule', { position: 'top-center' });
			onSaved();
		} catch (e: unknown) {
			error = e instanceof Error ? e.message : 'Failed to save layers';
		} finally {
			loading = false;
		}
	}

	onMount(async () => {
		layers = JSON.parse(JSON.stringify(schedule.definition?.layers ?? []));
		try {
			members = await oncallState.getMembers(organizationId);
		} catch {
			members = [];
		}
	});
</script>

<div class="space-y-4 rounded-md border bg-muted/30 p-4">
	<div class="space-y-1">
		<h3 class="text-sm font-medium">Layers</h3>
		<p class="text-xs text-muted-foreground">
			Later layers take precedence over earlier ones. If a handoff falls inside a restriction
			window, the person changes mid-window.
		</p>
	</div>

	<ErrorAlert {error} />

	{#each layers as layer, layerIndex (layerIndex)}
		<div class="space-y-3 rounded-md border bg-background p-3">
			<div class="flex items-center gap-2">
				<Input bind:value={layer.name} placeholder="Layer name" class="max-w-56" />
				<div class="flex-1"></div>
				<Button
					variant="ghost"
					size="icon"
					class="h-7 w-7"
					disabled={layerIndex === 0}
					onclick={() => moveLayer(layerIndex, -1)}
				>
					<ChevronUp class="h-4 w-4" />
				</Button>
				<Button
					variant="ghost"
					size="icon"
					class="h-7 w-7"
					disabled={layerIndex === layers.length - 1}
					onclick={() => moveLayer(layerIndex, 1)}
				>
					<ChevronDown class="h-4 w-4" />
				</Button>
				<Button
					variant="ghost"
					size="icon"
					class="h-7 w-7"
					title="Remove layer"
					onclick={() => removeLayer(layerIndex)}
				>
					<Trash2 class="h-4 w-4" />
				</Button>
			</div>

			<div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
				<div class="space-y-1">
					<Label class="text-xs">Rotation</Label>
					<Select.Root
						type="single"
						value={layer.rotationType}
						onValueChange={(val) => {
							if (val) layer.rotationType = val as ScheduleLayer['rotationType'];
						}}
					>
						<Select.Trigger class="w-full">
							{rotationOptions.find((o) => o.value === layer.rotationType)?.label}
						</Select.Trigger>
						<Select.Content>
							{#each rotationOptions as option (option.value)}
								<Select.Item value={option.value}>{option.label}</Select.Item>
							{/each}
						</Select.Content>
					</Select.Root>
				</div>
				<div class="space-y-1">
					<Label class="text-xs">Handoff time</Label>
					<Input type="time" bind:value={layer.handoffTime} />
				</div>
				{#if layer.rotationType === 'weekly'}
					<div class="space-y-1">
						<Label class="text-xs">Handoff day</Label>
						<Select.Root
							type="single"
							value={layer.handoffDay !== undefined ? String(layer.handoffDay) : undefined}
							onValueChange={(val) => {
								if (val) layer.handoffDay = Number(val);
							}}
						>
							<Select.Trigger class="w-full">{dayLabel(layer.handoffDay)}</Select.Trigger>
							<Select.Content>
								{#each dayOptions as option (option.value)}
									<Select.Item value={String(option.value)}>{option.label}</Select.Item>
								{/each}
							</Select.Content>
						</Select.Root>
					</div>
				{:else if layer.rotationType === 'custom'}
					<div class="space-y-1">
						<Label class="text-xs">Interval (days)</Label>
						<Input type="number" min={1} bind:value={layer.intervalDays} />
					</div>
				{/if}
				<div class="space-y-1">
					<Label class="text-xs">Rotation start</Label>
					<Input type="date" bind:value={layer.rotationStart} />
				</div>
			</div>

			<div class="space-y-1">
				<Label class="text-xs">Members (rotation order)</Label>
				{#each layer.userIds as userId, memberIndex (memberIndex)}
					<div class="flex items-center gap-2">
						<span class="w-5 text-xs text-muted-foreground">{memberIndex + 1}.</span>
						<span class="flex-1 truncate text-sm">{memberLabel(userId)}</span>
						<Button
							variant="ghost"
							size="icon"
							class="h-6 w-6"
							disabled={memberIndex === 0}
							onclick={() => moveLayerMember(layer, memberIndex, -1)}
						>
							<ChevronUp class="h-3.5 w-3.5" />
						</Button>
						<Button
							variant="ghost"
							size="icon"
							class="h-6 w-6"
							disabled={memberIndex === layer.userIds.length - 1}
							onclick={() => moveLayerMember(layer, memberIndex, 1)}
						>
							<ChevronDown class="h-3.5 w-3.5" />
						</Button>
						<Button
							variant="ghost"
							size="icon"
							class="h-6 w-6"
							onclick={() => removeLayerMember(layer, memberIndex)}
						>
							<X class="h-3.5 w-3.5" />
						</Button>
					</div>
				{:else}
					<p class="text-xs text-muted-foreground">No members in this layer yet.</p>
				{/each}
				{#if members.filter((m) => !layer.userIds.includes(m.id)).length > 0}
					<Select.Root
						type="single"
						value={undefined}
						onValueChange={(val) => {
							if (val) addLayerMember(layer, Number(val));
						}}
					>
						<Select.Trigger class="w-64">Add member...</Select.Trigger>
						<Select.Content>
							{#each members.filter((m) => !layer.userIds.includes(m.id)) as member (member.id)}
								<Select.Item value={String(member.id)}>
									{member.name || member.email}
								</Select.Item>
							{/each}
						</Select.Content>
					</Select.Root>
				{/if}
			</div>

			<div class="space-y-1">
				<Label class="text-xs">Restrictions</Label>
				{#each layer.restrictions as restriction, restrictionIndex (restrictionIndex)}
					<div class="flex flex-wrap items-center gap-2">
						<Select.Root
							type="single"
							value={restriction.type}
							onValueChange={(val) => {
								if (val) setRestrictionType(restriction, val as 'daily' | 'weekly');
							}}
						>
							<Select.Trigger class="w-28">
								{restriction.type === 'daily' ? 'Daily' : 'Weekly'}
							</Select.Trigger>
							<Select.Content>
								<Select.Item value="daily">Daily</Select.Item>
								<Select.Item value="weekly">Weekly</Select.Item>
							</Select.Content>
						</Select.Root>
						{#if restriction.type === 'weekly'}
							<Select.Root
								type="single"
								value={restriction.startDay !== undefined ? String(restriction.startDay) : undefined}
								onValueChange={(val) => {
									if (val) restriction.startDay = Number(val);
								}}
							>
								<Select.Trigger class="w-32">{dayLabel(restriction.startDay)}</Select.Trigger>
								<Select.Content>
									{#each dayOptions as option (option.value)}
										<Select.Item value={String(option.value)}>{option.label}</Select.Item>
									{/each}
								</Select.Content>
							</Select.Root>
						{/if}
						<Input type="time" bind:value={restriction.startTime} class="w-32" />
						<span class="text-xs text-muted-foreground">to</span>
						{#if restriction.type === 'weekly'}
							<Select.Root
								type="single"
								value={restriction.endDay !== undefined ? String(restriction.endDay) : undefined}
								onValueChange={(val) => {
									if (val) restriction.endDay = Number(val);
								}}
							>
								<Select.Trigger class="w-32">{dayLabel(restriction.endDay)}</Select.Trigger>
								<Select.Content>
									{#each dayOptions as option (option.value)}
										<Select.Item value={String(option.value)}>{option.label}</Select.Item>
									{/each}
								</Select.Content>
							</Select.Root>
						{/if}
						<Input type="time" bind:value={restriction.endTime} class="w-32" />
						<Button
							variant="ghost"
							size="icon"
							class="h-6 w-6"
							onclick={() => removeRestriction(layer, restrictionIndex)}
						>
							<X class="h-3.5 w-3.5" />
						</Button>
					</div>
				{:else}
					<p class="text-xs text-muted-foreground">No restrictions. The layer covers all hours.</p>
				{/each}
				<Button variant="outline" size="sm" onclick={() => addRestriction(layer)}>
					<Plus class="mr-1 h-3 w-3" /> Add Restriction
				</Button>
			</div>
		</div>
	{/each}

	<div class="flex items-center justify-between">
		<Button variant="outline" size="sm" onclick={addLayer}>
			<Plus class="mr-1 h-4 w-4" /> Add Layer
		</Button>
		<div class="flex gap-2">
			<Button variant="outline" onclick={onCancel} disabled={loading}>Cancel</Button>
			<Button onclick={handleSave} disabled={loading}>
				<Check class="mr-2 h-4 w-4" />
				{loading ? 'Updating...' : 'Update Schedule'}
			</Button>
		</div>
	</div>
</div>
