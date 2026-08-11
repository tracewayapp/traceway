<script lang="ts">
	import { onMount } from 'svelte';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import * as Select from '$lib/components/ui/select';
	import { ErrorAlert } from '$lib/components/ui/error-alert';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { Plus, Check, Trash2, ChevronUp, ChevronDown } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';
	import { api } from '$lib/api';
	import { type ContactMethod, type UserNotificationRuleStep } from '$lib/state/oncall.svelte';

	interface StepDraft {
		contactMethodId: number | null;
		delayMinutes: number;
	}

	let methods = $state<ContactMethod[]>([]);
	let highSteps = $state<StepDraft[]>([]);
	let lowSteps = $state<StepDraft[]>([]);
	let loading = $state(true);
	let saving = $state(false);
	let error = $state('');
	let loadError = $state('');

	onMount(() => {
		load();
	});

	async function load() {
		loading = true;
		loadError = '';
		try {
			const [methodsRes, rulesRes] = await Promise.all([
				api.get('/contact-methods'),
				api.get('/user-notification-rules')
			]);
			methods = methodsRes.methods || [];
			highSteps = toDrafts(rulesRes.high);
			lowSteps = toDrafts(rulesRes.low);
		} catch (e: unknown) {
			// Never render a failed load as "no steps": saving from that state
			// would wipe the chains the server still holds.
			loadError = e instanceof Error ? e.message : 'Failed to load notification rules';
			methods = [];
			highSteps = [];
			lowSteps = [];
		} finally {
			loading = false;
		}
	}

	function toDrafts(steps: UserNotificationRuleStep[] | undefined): StepDraft[] {
		return (steps ?? []).map((s) => ({
			contactMethodId: s.contactMethodId,
			delayMinutes: s.delayMinutes
		}));
	}

	function methodLabel(method: ContactMethod): string {
		const config = method.config || {};
		if (method.methodType === 'email') {
			return `Email (${config.email || 'account'})`;
		}
		if (method.methodType === 'slack') {
			try {
				return `Slack (${new URL(config.webhookUrl).host})`;
			} catch {
				return 'Slack (webhook)';
			}
		}
		if (method.methodType === 'pushover') {
			const key = config.userKey || '';
			return `Pushover (${key.length > 4 ? `${key.slice(0, 4)}…` : key || '—'})`;
		}
		if (method.methodType === 'telegram') {
			return `Telegram (chat ${config.chatId || '—'})`;
		}
		if (method.methodType === 'sms') {
			return `SMS ${config.phoneNumber || ''}`.trim();
		}
		return method.methodType;
	}

	function methodUnavailable(method: ContactMethod): string {
		if (method.methodType === 'sms' && !method.verified) return '(verify first)';
		if (!method.enabled) return '(disabled)';
		return '';
	}

	function selectedLabel(step: StepDraft): string {
		if (step.contactMethodId === null) return 'Select a method...';
		const method = methods.find((m) => m.id === step.contactMethodId);
		if (!method) return `Method #${step.contactMethodId}`;
		const unavailable = methodUnavailable(method);
		return unavailable ? `${methodLabel(method)} ${unavailable}` : methodLabel(method);
	}

	function addStep(steps: StepDraft[]): StepDraft[] {
		if (steps.length >= 10) return steps;
		const firstAvailable = methods.find((m) => !methodUnavailable(m));
		return [...steps, { contactMethodId: firstAvailable?.id ?? null, delayMinutes: 0 }];
	}

	function removeStep(steps: StepDraft[], index: number): StepDraft[] {
		return steps.filter((_, i) => i !== index);
	}

	function moveStep(steps: StepDraft[], index: number, direction: -1 | 1): StepDraft[] {
		const target = index + direction;
		if (target < 0 || target >= steps.length) return steps;
		const next = [...steps];
		[next[index], next[target]] = [next[target], next[index]];
		return next;
	}

	function toPayload(steps: StepDraft[]) {
		return steps.map((s) => ({
			contactMethodId: s.contactMethodId ?? 0,
			delayMinutes: Number(s.delayMinutes) || 0
		}));
	}

	async function handleSubmit() {
		saving = true;
		error = '';
		try {
			const res = await api.put('/user-notification-rules', {
				high: toPayload(highSteps),
				low: toPayload(lowSteps)
			});
			highSteps = toDrafts(res.high);
			lowSteps = toDrafts(res.low);
			toast.success('Successfully updated the Notification Rules', { position: 'top-center' });
		} catch (e: unknown) {
			error = e instanceof Error ? e.message : 'Failed to update notification rules';
		} finally {
			saving = false;
		}
	}
</script>

{#snippet chain(title: string, steps: StepDraft[], setSteps: (next: StepDraft[]) => void)}
	<div class="space-y-2">
		<h3 class="text-sm font-medium">{title}</h3>
		{#if steps.length === 0}
			<p class="text-sm text-muted-foreground">
				No steps — all your contact methods are notified immediately.
			</p>
		{/if}
		<div class="space-y-2">
			{#each steps as step, index (index)}
				<div class="flex flex-wrap items-center gap-2 rounded-md border p-2">
					<span class="text-sm text-muted-foreground">{index + 1}.</span>
					<span class="text-sm">Notify</span>
					<Select.Root
						type="single"
						value={step.contactMethodId !== null ? String(step.contactMethodId) : undefined}
						onValueChange={(val) => {
							if (val) {
								step.contactMethodId = Number(val);
								setSteps([...steps]);
							}
						}}
					>
						<Select.Trigger class="w-[220px]">
							{selectedLabel(step)}
						</Select.Trigger>
						<Select.Content>
							{#each methods as method (method.id)}
								{@const unavailable = methodUnavailable(method)}
								<Select.Item value={String(method.id)} disabled={unavailable !== ''}>
									{methodLabel(method)}{unavailable ? ` ${unavailable}` : ''}
								</Select.Item>
							{/each}
						</Select.Content>
					</Select.Root>
					<span class="text-sm">after</span>
					<Input
						type="number"
						min={0}
						max={120}
						class="w-20"
						aria-label="Delay in minutes"
						bind:value={step.delayMinutes}
					/>
					<span class="text-sm">minutes</span>
					<div class="ml-auto flex gap-1">
						<Button
							variant="ghost"
							size="icon"
							type="button"
							title="Move up"
							disabled={index === 0}
							onclick={() => setSteps(moveStep(steps, index, -1))}
						>
							<ChevronUp class="h-4 w-4" />
						</Button>
						<Button
							variant="ghost"
							size="icon"
							type="button"
							title="Move down"
							disabled={index === steps.length - 1}
							onclick={() => setSteps(moveStep(steps, index, 1))}
						>
							<ChevronDown class="h-4 w-4" />
						</Button>
						<Button
							variant="ghost"
							size="icon"
							type="button"
							title="Remove step"
							onclick={() => setSteps(removeStep(steps, index))}
						>
							<Trash2 class="h-4 w-4" />
						</Button>
					</div>
				</div>
			{/each}
		</div>
		<Button
			variant="outline"
			size="sm"
			type="button"
			disabled={steps.length >= 10}
			onclick={() => setSteps(addStep(steps))}
		>
			<Plus class="mr-1 h-3 w-3" /> Add step
		</Button>
	</div>
{/snippet}

<div class="flex items-center justify-between">
	<h2 class="text-xl font-semibold tracking-tight">Notification Rules</h2>
</div>

<p class="text-sm text-muted-foreground">
	When a page targets you, these steps run for its urgency until you acknowledge. With no steps,
	all your contact methods are notified immediately.
</p>

{#if loading}
	<div class="flex justify-center py-12"><LoadingCircle size="lg" /></div>
{:else if loadError}
	<div class="flex flex-col items-center justify-center gap-3 rounded-md bg-muted py-12 text-center">
		<p class="text-sm text-destructive">{loadError}</p>
		<Button variant="outline" size="sm" onclick={() => load()}>Retry</Button>
	</div>
{:else}
	<div class="space-y-6">
		<ErrorAlert {error} />

		{@render chain('High urgency', highSteps, (next) => (highSteps = next))}
		{@render chain('Low urgency', lowSteps, (next) => (lowSteps = next))}

		<div>
			<Button onclick={handleSubmit} disabled={saving}>
				<Check class="mr-2 h-4 w-4" />
				{saving ? 'Updating...' : 'Update Notification Rules'}
			</Button>
		</div>
	</div>
{/if}
