<script lang="ts">
	import { Calendar, User, Users, Bell, ArrowRight, Repeat2 } from '@lucide/svelte';
	import type { PolicyDefinition, PolicyTarget } from '$lib/state/oncall.svelte';

	interface Props {
		definition: PolicyDefinition;
		labels?: Record<string, string>;
		currentLevel?: number | null;
		compact?: boolean;
	}

	let { definition, labels = {}, currentLevel = null, compact = false }: Props = $props();

	const typeIcons = {
		schedule: Calendar,
		user: User,
		team: Users,
		channel: Bell
	};

	const steps = $derived(definition?.steps ?? []);
	const repeatCount = $derived(definition?.repeatCount ?? 0);

	function targetLabel(target: PolicyTarget): string {
		return labels[`${target.type}:${target.id}`] ?? `${target.type} #${target.id}`;
	}
</script>

<div class="flex flex-wrap items-center {compact ? 'gap-1.5' : 'gap-2'}">
	{#each steps as step, i (i)}
		{#if i > 0}
			<div
				class="flex items-center gap-1 whitespace-nowrap text-muted-foreground {compact
					? 'text-[10px]'
					: 'text-xs'}"
			>
				<ArrowRight class={compact ? 'h-3 w-3' : 'h-3.5 w-3.5'} />
				<span>after {steps[i - 1].delayMinutes}m</span>
			</div>
		{/if}
		<div
			class="rounded-md border bg-card {compact ? 'px-2 py-1' : 'px-3 py-2'} {currentLevel === i
				? 'border-primary ring-1 ring-primary'
				: ''}"
		>
			<div
				class="font-medium text-muted-foreground {compact ? 'mb-0.5 text-[10px]' : 'mb-1 text-xs'}"
			>
				L{i + 1}
			</div>
			<div class="flex flex-wrap items-center gap-1">
				{#each step.targets as target (target.type + ':' + target.id)}
					{@const Icon = typeIcons[target.type] ?? Bell}
					<span
						class="inline-flex items-center gap-1 rounded-full bg-muted text-foreground {compact
							? 'px-1.5 py-0.5 text-[10px]'
							: 'px-2 py-0.5 text-xs'}"
					>
						<Icon class="h-3 w-3 shrink-0 text-muted-foreground" />
						{targetLabel(target)}
					</span>
				{/each}
				{#if step.targets.length === 0}
					<span class="text-muted-foreground {compact ? 'text-[10px]' : 'text-xs'}">No targets</span>
				{/if}
			</div>
		</div>
	{/each}
	{#if steps.length === 0}
		<span class="text-muted-foreground {compact ? 'text-[10px]' : 'text-xs'}">No steps</span>
	{/if}
	{#if repeatCount > 0}
		<span
			class="inline-flex items-center gap-1 whitespace-nowrap text-muted-foreground {compact
				? 'text-[10px]'
				: 'text-xs'}"
		>
			<Repeat2 class={compact ? 'h-3 w-3' : 'h-3.5 w-3.5'} />
			repeats ×{repeatCount}
		</span>
	{/if}
</div>
