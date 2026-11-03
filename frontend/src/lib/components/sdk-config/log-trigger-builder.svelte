<script lang="ts">
	import * as Select from '$lib/components/ui/select';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Checkbox } from '$lib/components/ui/checkbox';
	import { ScrollText } from '@lucide/svelte';
	import { logLineTrigger, type LogMatchOp } from './sdk-config';

	interface Props {
		disabled?: boolean;
		onAdd: (trigger: Record<string, unknown>) => void;
	}

	let { disabled = false, onAdd }: Props = $props();

	const opLabels: Record<LogMatchOp, string> = {
		icontains: 'contains (any case)',
		glob: 'matches pattern (* ?)',
		starts_with: 'starts with'
	};
	const levelLabels: Record<string, string> = {
		'0': 'any level',
		'13': 'warn or worse',
		'17': 'error or worse'
	};
	const lookbackLabels: Record<string, string> = {
		'60': 'last minute',
		'300': 'last 5 minutes',
		'600': 'last 10 minutes'
	};

	let text = $state('');
	let op = $state<LogMatchOp>('icontains');
	let level = $state('0');
	let count = $state('1');
	let periodSeconds = $state('60');
	let durationMinutes = $state('30');
	let lookback = $state('300');
	let replay = $state(true);
	let error = $state('');

	function add() {
		const n = Math.floor(Number(count));
		const period = Number(periodSeconds);
		const minutes = Number(durationMinutes);
		if (!text.trim()) {
			error = 'Enter the text or pattern of the log line.';
			return;
		}
		if (!(n >= 1 && n <= 1000) || !(minutes >= 1 && minutes <= 1440) || (n > 1 && !(period >= 1))) {
			error = 'Times must be 1-1000, the window at least 1 s, and the capture 1-1440 minutes.';
			return;
		}
		error = '';
		onAdd(
			logLineTrigger({
				text: text.trim(),
				op,
				minSeverity: Number(level),
				count: n,
				periodSeconds: period,
				durationMinutes: minutes,
				lookbackSeconds: Number(lookback),
				replay
			})
		);
		text = '';
	}
</script>

<div class="space-y-3 rounded-md border p-3">
	<div class="flex items-center gap-2 text-sm font-medium">
		<ScrollText class="h-4 w-4" />
		Capture when a log line appears
	</div>
	<p class="text-xs text-muted-foreground">
		Matched on the device against every log record, including lines that are not collected. The
		capture uploads what the device buffered and then streams everything, also on the following
		pages, until it ends.
	</p>
	<div class="grid gap-2 sm:grid-cols-[auto_1fr]">
		<Select.Root type="single" bind:value={op} {disabled}>
			<Select.Trigger class="w-48">{opLabels[op]}</Select.Trigger>
			<Select.Content>
				{#each Object.entries(opLabels) as [value, label] (value)}
					<Select.Item {value}>{label}</Select.Item>
				{/each}
			</Select.Content>
		</Select.Root>
		<Input
			placeholder={op === 'glob' ? 'payment * failed for order *' : 'payment failed'}
			bind:value={text}
			{disabled}
		/>
	</div>
	<div class="flex flex-wrap items-center gap-2 text-sm">
		<Select.Root type="single" bind:value={level} {disabled}>
			<Select.Trigger class="w-40">{levelLabels[level]}</Select.Trigger>
			<Select.Content>
				{#each Object.entries(levelLabels) as [value, label] (value)}
					<Select.Item {value}>{label}</Select.Item>
				{/each}
			</Select.Content>
		</Select.Root>
		<Input class="w-20" type="number" min="1" max="1000" bind:value={count} {disabled} />
		<span class="text-muted-foreground">time(s)</span>
		{#if Number(count) > 1}
			<span class="text-muted-foreground">within</span>
			<Input class="w-20" type="number" min="1" bind:value={periodSeconds} {disabled} />
			<span class="text-muted-foreground">s</span>
		{/if}
	</div>
	<div class="flex flex-wrap items-center gap-2 text-sm">
		<span class="text-muted-foreground">capture for</span>
		<Input class="w-20" type="number" min="1" max="1440" bind:value={durationMinutes} {disabled} />
		<span class="text-muted-foreground">min, including the</span>
		<Select.Root type="single" bind:value={lookback} {disabled}>
			<Select.Trigger class="w-40">{lookbackLabels[lookback]}</Select.Trigger>
			<Select.Content>
				{#each Object.entries(lookbackLabels) as [value, label] (value)}
					<Select.Item {value}>{label}</Select.Item>
				{/each}
			</Select.Content>
		</Select.Root>
		<Checkbox
			checked={replay}
			onCheckedChange={(checked) => (replay = checked === true)}
			{disabled}
			aria-label="Record session replay"
		/>
		<Label
			class="cursor-pointer font-normal"
			onclick={() => {
				if (!disabled) replay = !replay;
			}}>with replay</Label
		>
	</div>
	{#if error}
		<p class="text-xs text-destructive">{error}</p>
	{/if}
	<Button type="button" variant="outline" size="sm" onclick={add} {disabled}>Add trigger</Button>
</div>
