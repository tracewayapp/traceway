<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Plus, Trash2, ArrowUp, ArrowDown } from '@lucide/svelte';
	import { newTriggerId } from './sdk-config';
	type Step = {
		match: {
			event: {
				type: 'click' | 'hover' | 'navigation' | 'log';
				value: string;
				mode?: 'exact' | 'contains';
			};
		};
		withinSeconds?: number;
	};
	type Sequence = {
		id: string;
		name: string;
		enabled?: boolean;
		steps: Step[];
		actions: [
			{
				type: 'capture';
				replay: true;
				lookbackSeconds: number;
				durationSeconds: number;
				signals: string[];
			}
		];
		cooldownSeconds: number;
		maxPerSession: number;
	};
	let {
		triggers = $bindable<unknown[]>([]),
		disabled = false
	}: { triggers?: unknown[]; disabled?: boolean } = $props();
	function sequence(value: unknown): value is Sequence {
		const t = value as Sequence;
		return (
			!!t &&
			Array.isArray(t.steps) &&
			t.steps.every((s) => !!s.match?.event) &&
			t.actions?.length === 1 &&
			t.actions[0].type === 'capture' &&
			t.actions[0].replay === true
		);
	}
	function add() {
		triggers = [
			...triggers,
			{
				id: newTriggerId(),
				name: 'User journey',
				steps: [{ match: { event: { type: 'click', value: '' } } }],
				actions: [
					{
						type: 'capture',
						replay: true,
						lookbackSeconds: 60,
						durationSeconds: 30,
						signals: ['console', 'navigation', 'userActions', 'network']
					}
				],
				cooldownSeconds: 60,
				maxPerSession: 3
			}
		];
	}
	function move(t: Sequence, i: number, delta: number) {
		const steps = [...t.steps];
		[steps[i], steps[i + delta]] = [steps[i + delta], steps[i]];
		t.steps = steps;
	}
</script>

<div class="space-y-4 rounded-lg border p-4" data-testid="sequence-builder">
	<div>
		<h3 class="font-medium">Recording triggers</h3>
		<p class="text-xs text-muted-foreground">
			Match events in order on the same tab, then send buffered replay and continue recording.
			Unrelated events are ignored, and the first step happening again does not undo progress.
		</p>
	</div>
	{#each triggers as item, index (item)}
		{#if sequence(item)}
			<div class="space-y-3 rounded-md border p-3" data-testid="sequence-trigger">
				<div class="flex items-end gap-2">
					<div class="flex-1 space-y-1">
						<Label for={`sequence-name-${index}`}>Trigger name</Label><Input
							id={`sequence-name-${index}`}
							bind:value={item.name}
							{disabled}
						/>
					</div>
					<Button
						type="button"
						variant="ghost"
						size="icon"
						aria-label="Remove trigger"
						onclick={() => (triggers = triggers.filter((_, i) => i !== index))}
						{disabled}><Trash2 class="size-4" /></Button
					>
				</div>
				{#each item.steps as step, i (step)}
					<div class="space-y-2 rounded border bg-muted/20 p-3" data-testid="sequence-step">
						<div class="flex items-center gap-2">
							<span class="text-sm font-medium">Step {i + 1}</span>
							<div class="ml-auto flex gap-1">
								{#if i > 0}<Button
										type="button"
										variant="ghost"
										size="icon"
										aria-label={`Move step ${i + 1} up`}
										onclick={() => move(item, i, -1)}
										{disabled}><ArrowUp class="size-3" /></Button
									>{/if}
								{#if i < item.steps.length - 1}<Button
										type="button"
										variant="ghost"
										size="icon"
										aria-label={`Move step ${i + 1} down`}
										onclick={() => move(item, i, 1)}
										{disabled}><ArrowDown class="size-3" /></Button
									>{/if}
								{#if item.steps.length > 1}<Button
										type="button"
										variant="ghost"
										size="icon"
										aria-label={`Remove step ${i + 1}`}
										onclick={() => (item.steps = item.steps.filter((_, n) => n !== i))}
										{disabled}><Trash2 class="size-3" /></Button
									>{/if}
							</div>
						</div>
						<div class="grid gap-2 sm:grid-cols-[140px_1fr]">
							<div class="space-y-1">
								<Label for={`seq-type-${index}-${i}`}>Event</Label><select
									id={`seq-type-${index}-${i}`}
									class="h-9 w-full rounded-md border bg-background px-2 text-sm"
									bind:value={step.match.event.type}
									{disabled}
									><option value="click">Click</option><option value="hover">Hover</option><option
										value="navigation">Navigation</option
									><option value="log">Log</option></select
								>
							</div>
							<div class="space-y-1">
								<Label for={`seq-value-${index}-${i}`}
									>{step.match.event.type === 'navigation'
										? 'Route pattern'
										: step.match.event.type === 'log'
											? 'Log message'
											: 'CSS selector'}</Label
								><Input
									id={`seq-value-${index}-${i}`}
									bind:value={step.match.event.value}
									placeholder={step.match.event.type === 'navigation'
										? '/users/{id}'
										: step.match.event.type === 'log'
											? 'User loaded'
											: '#someId .some-class'}
									{disabled}
								/>
							</div>
						</div>
						{#if step.match.event.type === 'log'}<div class="space-y-1">
								<Label for={`seq-mode-${index}-${i}`}>Message matching</Label><select
									id={`seq-mode-${index}-${i}`}
									class="h-9 rounded-md border bg-background px-2 text-sm"
									value={step.match.event.mode ?? 'exact'}
									onchange={(e) =>
										(step.match.event.mode = e.currentTarget.value as 'exact' | 'contains')}
									{disabled}
									><option value="exact">Exactly matches</option><option value="contains"
										>Contains</option
									></select
								>
							</div>{/if}
						{#if i > 0}<div class="space-y-1">
								<Label for={`seq-within-${index}-${i}`}>Within seconds of previous step</Label
								><Input
									id={`seq-within-${index}-${i}`}
									type="number"
									min="1"
									max="300"
									bind:value={step.withinSeconds}
									{disabled}
								/>
							</div>{/if}
					</div>
				{/each}
				{#if item.steps.length < 5}<Button
						type="button"
						variant="outline"
						size="sm"
						onclick={() =>
							(item.steps = [
								...item.steps,
								{ match: { event: { type: 'navigation', value: '' } }, withinSeconds: 30 }
							])}
						{disabled}><Plus class="size-3" /> Add step</Button
					>{/if}
				<div class="grid gap-3 sm:grid-cols-2">
					<div class="space-y-1">
						<Label for={`seq-before-${index}`}>Replay lookback (seconds)</Label><Input
							id={`seq-before-${index}`}
							type="number"
							min="1"
							max="86400"
							bind:value={item.actions[0].lookbackSeconds}
							{disabled}
						/>
					</div>
					<div class="space-y-1">
						<Label for={`seq-after-${index}`}>Continue recording (seconds)</Label><Input
							id={`seq-after-${index}`}
							type="number"
							min="1"
							max="86400"
							bind:value={item.actions[0].durationSeconds}
							{disabled}
						/>
					</div>
					<div class="space-y-1">
						<Label for={`seq-cooldown-${index}`}>Cooldown (seconds)</Label><Input
							id={`seq-cooldown-${index}`}
							type="number"
							min="0"
							bind:value={item.cooldownSeconds}
							{disabled}
						/>
					</div>
					<div class="space-y-1">
						<Label for={`seq-limit-${index}`}>Captures per session</Label><Input
							id={`seq-limit-${index}`}
							type="number"
							min="0"
							bind:value={item.maxPerSession}
							{disabled}
						/>
					</div>
				</div>
			</div>
		{:else}<p class="text-xs text-muted-foreground">
				An existing advanced trigger is preserved. Use the JSON editor to change it.
			</p>{/if}
	{/each}
	{#if triggers.length < 20}<Button type="button" variant="outline" onclick={add} {disabled}
			><Plus class="size-4" /> Add recording trigger</Button
		>{/if}
	<p class="text-xs text-muted-foreground">
		Selectors support tags, IDs, classes, attributes and combinators; pseudo-classes and selector
		lists are not supported. Hover fires when the pointer enters a matching element. Route
		parameters match one path segment. Replay remains subject to consent, developer limits and
		available buffer.
	</p>
</div>
