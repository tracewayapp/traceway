<script lang="ts">
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Select from '$lib/components/ui/select';
	import { ErrorAlert } from '$lib/components/ui/error-alert';
	import {
		Plus,
		Check,
		Trash2,
		X,
		ChevronUp,
		ChevronDown,
		Calendar,
		User,
		Users,
		Bell
	} from '@lucide/svelte';
	import { toast } from 'svelte-sonner';
	import {
		oncallState,
		type EscalationPolicy,
		type PolicyDefinition,
		type PolicyTarget,
		type PolicyTargetType
	} from '$lib/state/oncall.svelte';
	import EscalationChain from '$lib/components/traceway/escalation-chain.svelte';

	interface TargetOption {
		id: number;
		label: string;
	}

	interface Props {
		open: boolean;
		organizationId: number;
		policy: EscalationPolicy | null;
		targetOptions: Record<PolicyTargetType, TargetOption[]>;
		labels: Record<string, string>;
		onSaved: () => void;
	}

	let {
		open = $bindable(),
		organizationId,
		policy,
		targetOptions,
		labels,
		onSaved
	}: Props = $props();

	interface StepDraft {
		delayMinutes: number;
		targets: PolicyTarget[];
		addType: PolicyTargetType;
	}

	let name = $state('');
	let steps = $state<StepDraft[]>([]);
	let repeatCount = $state(0);
	let urgency = $state<'auto' | 'high' | 'low'>('auto');
	let loading = $state(false);
	let error = $state('');

	const urgencyOptions: { value: 'auto' | 'high' | 'low'; label: string }[] = [
		{ value: 'auto', label: 'Auto — critical pages are high urgency (default)' },
		{ value: 'high', label: 'Always high' },
		{ value: 'low', label: 'Always low' }
	];

	const isEditing = $derived(policy !== null);

	const typeLabels: Record<PolicyTargetType, string> = {
		schedule: 'Schedule',
		user: 'User',
		team: 'Team',
		channel: 'Channel'
	};

	const typeIcons = {
		schedule: Calendar,
		user: User,
		team: Users,
		channel: Bell
	};

	const previewDefinition = $derived<PolicyDefinition>({
		schemaVersion: 1,
		steps: steps.map((s) => ({
			targets: s.targets,
			delayMinutes: Number(s.delayMinutes) || 0
		})),
		repeatCount: Number(repeatCount) || 0,
		urgency
	});

	$effect(() => {
		if (open) {
			if (policy) {
				name = policy.name;
				steps = (policy.definition?.steps ?? []).map((s) => ({
					delayMinutes: s.delayMinutes,
					targets: [...s.targets],
					addType: 'schedule'
				}));
				repeatCount = policy.definition?.repeatCount ?? 0;
				urgency = policy.definition?.urgency || 'auto';
			} else {
				name = '';
				steps = [{ delayMinutes: 5, targets: [], addType: 'schedule' }];
				repeatCount = 0;
				urgency = 'auto';
			}
			error = '';
		}
	});

	function targetLabel(target: PolicyTarget): string {
		return labels[`${target.type}:${target.id}`] ?? `${target.type} #${target.id}`;
	}

	function addStep() {
		steps = [...steps, { delayMinutes: 5, targets: [], addType: 'schedule' }];
	}

	function removeStep(index: number) {
		steps = steps.filter((_, i) => i !== index);
	}

	function moveStep(index: number, direction: -1 | 1) {
		const target = index + direction;
		if (target < 0 || target >= steps.length) return;
		const next = [...steps];
		[next[index], next[target]] = [next[target], next[index]];
		steps = next;
	}

	function availableOptions(step: StepDraft): TargetOption[] {
		return (targetOptions[step.addType] ?? []).filter(
			(option) => !step.targets.some((t) => t.type === step.addType && t.id === option.id)
		);
	}

	function addTarget(index: number, id: number) {
		const step = steps[index];
		if (step.targets.some((t) => t.type === step.addType && t.id === id)) return;
		step.targets = [...step.targets, { type: step.addType, id }];
		steps = [...steps];
	}

	function removeTarget(index: number, target: PolicyTarget) {
		const step = steps[index];
		step.targets = step.targets.filter((t) => !(t.type === target.type && t.id === target.id));
		steps = [...steps];
	}

	async function handleSubmit() {
		loading = true;
		error = '';
		try {
			const body = { name, definition: previewDefinition };
			if (isEditing && policy) {
				await oncallState.updatePolicy(organizationId, policy.id, body);
				toast.success('Successfully updated the Policy', { position: 'top-center' });
			} else {
				await oncallState.createPolicy(organizationId, body);
				toast.success('Successfully created the Policy', { position: 'top-center' });
			}
			onSaved();
		} catch (e: unknown) {
			error = e instanceof Error ? e.message : 'Failed to save policy';
		} finally {
			loading = false;
		}
	}
</script>

<AlertDialog.Root {open} onOpenChange={(isOpen) => (open = isOpen)}>
	<AlertDialog.Content class="max-h-[90vh] overflow-y-auto sm:max-w-2xl">
		<AlertDialog.Header>
			<AlertDialog.Title>{isEditing ? 'Edit Policy' : 'New Policy'}</AlertDialog.Title>
			<AlertDialog.Description>
				{isEditing
					? 'Update the escalation policy steps and targets'
					: 'Define who gets paged, and in what order'}
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
				<Label for="policy-name">Name</Label>
				<Input id="policy-name" bind:value={name} placeholder="e.g. Backend escalation" />
			</div>

			<div class="space-y-2">
				<Label>Steps</Label>
				<div class="space-y-3">
					{#each steps as step, index (index)}
						<div class="space-y-3 rounded-md border p-3">
							<div class="flex items-center justify-between">
								<span class="text-sm font-medium">Level {index + 1}</span>
								<div class="flex gap-1">
									<Button
										variant="ghost"
										size="icon"
										type="button"
										title="Move up"
										disabled={index === 0}
										onclick={() => moveStep(index, -1)}
									>
										<ChevronUp class="h-4 w-4" />
									</Button>
									<Button
										variant="ghost"
										size="icon"
										type="button"
										title="Move down"
										disabled={index === steps.length - 1}
										onclick={() => moveStep(index, 1)}
									>
										<ChevronDown class="h-4 w-4" />
									</Button>
									<Button
										variant="ghost"
										size="icon"
										type="button"
										title="Remove step"
										onclick={() => removeStep(index)}
									>
										<Trash2 class="h-4 w-4" />
									</Button>
								</div>
							</div>

							{#if step.targets.length > 0}
								<div class="flex flex-wrap gap-1">
									{#each step.targets as target (target.type + ':' + target.id)}
										{@const Icon = typeIcons[target.type] ?? Bell}
										<span
											class="inline-flex items-center gap-1 rounded-full bg-muted px-2 py-0.5 text-xs"
										>
											<Icon class="h-3 w-3 text-muted-foreground" />
											{targetLabel(target)}
											<button
												type="button"
												class="ml-0.5 cursor-pointer text-muted-foreground hover:text-foreground"
												title="Remove target"
												onclick={() => removeTarget(index, target)}
											>
												<X class="h-3 w-3" />
											</button>
										</span>
									{/each}
								</div>
							{/if}

							<div class="flex gap-2">
								<Select.Root
									type="single"
									value={step.addType}
									onValueChange={(val) => {
										if (val) {
											step.addType = val as PolicyTargetType;
											steps = [...steps];
										}
									}}
								>
									<Select.Trigger class="w-[130px]">
										{typeLabels[step.addType]}
									</Select.Trigger>
									<Select.Content>
										{#each Object.entries(typeLabels) as [value, label] (value)}
											<Select.Item {value}>{label}</Select.Item>
										{/each}
									</Select.Content>
								</Select.Root>
								{#key `${step.addType}-${step.targets.length}`}
									<Select.Root
										type="single"
										value={undefined}
										onValueChange={(val) => {
											if (val) addTarget(index, Number(val));
										}}
									>
										<Select.Trigger class="flex-1">
											{availableOptions(step).length > 0
												? `Add ${typeLabels[step.addType].toLowerCase()}...`
												: `No ${typeLabels[step.addType].toLowerCase()}s available`}
										</Select.Trigger>
										<Select.Content class="max-h-64 overflow-y-auto">
											{#each availableOptions(step) as option (option.id)}
												<Select.Item value={String(option.id)}>{option.label}</Select.Item>
											{/each}
										</Select.Content>
									</Select.Root>
								{/key}
							</div>

							<div class="flex items-center gap-2">
								<Label for="step-delay-{index}" class="text-xs font-normal whitespace-nowrap">
									Escalate after
								</Label>
								<Input
									id="step-delay-{index}"
									type="number"
									min={1}
									max={1440}
									class="w-20"
									bind:value={step.delayMinutes}
								/>
								<span class="text-xs text-muted-foreground">minutes if not acknowledged</span>
							</div>
						</div>
					{/each}
				</div>
				<Button variant="outline" size="sm" type="button" onclick={addStep}>
					<Plus class="mr-1 h-3 w-3" /> Add Step
				</Button>
			</div>

			<div class="space-y-2">
				<Label for="policy-repeat">Repeat</Label>
				<Input
					id="policy-repeat"
					type="number"
					min={0}
					max={5}
					class="w-20"
					bind:value={repeatCount}
				/>
				<p class="text-xs text-muted-foreground">
					After the last step, repeat the chain N more times.
				</p>
			</div>

			<div class="space-y-2">
				<Label>Urgency</Label>
				<Select.Root
					type="single"
					value={urgency}
					onValueChange={(val) => {
						if (val) urgency = val as 'auto' | 'high' | 'low';
					}}
				>
					<Select.Trigger class="w-full">
						{urgencyOptions.find((o) => o.value === urgency)?.label ?? 'Auto'}
					</Select.Trigger>
					<Select.Content>
						{#each urgencyOptions as option (option.value)}
							<Select.Item value={option.value}>{option.label}</Select.Item>
						{/each}
					</Select.Content>
				</Select.Root>
				<p class="text-xs text-muted-foreground">
					Urgency picks which of each responder's notification rule chains runs.
				</p>
			</div>

			<div class="space-y-2">
				<Label>Preview</Label>
				<div class="rounded-md bg-muted/50 p-3">
					<EscalationChain definition={previewDefinition} {labels} compact />
				</div>
			</div>
		</form>

		<AlertDialog.Footer>
			<AlertDialog.Cancel disabled={loading}>Cancel</AlertDialog.Cancel>
			<Button variant={isEditing ? 'default' : 'success'} onclick={handleSubmit} disabled={loading}>
				{#if isEditing}
					<Check class="mr-2 h-4 w-4" />
					{loading ? 'Updating...' : 'Update Policy'}
				{:else}
					<Plus class="mr-2 h-4 w-4" />
					{loading ? 'Creating...' : 'New Policy'}
				{/if}
			</Button>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
