<script lang="ts">
	import * as Popover from '$lib/components/ui/popover';
	import * as Select from '$lib/components/ui/select';
	import { cn } from '$lib/utils';
	import { Clock } from '@lucide/svelte';

	type Period = 'AM' | 'PM';

	interface Props {
		// Same wire format as <input type="time">: HH:mm (24-hour)
		value?: string;
		onValueChange?: (value: string) => void;
		disabled?: boolean;
		id?: string;
		class?: string;
		placeholder?: string;
	}

	let {
		value = $bindable(''),
		onValueChange,
		disabled = false,
		id,
		class: className,
		placeholder = 'Pick a time'
	}: Props = $props();

	let open = $state(false);

	let hours = $state(9);
	let minutes = $state(0);
	let period = $state<Period>('AM');

	const parsed = $derived.by(() => {
		const match = /^(\d{1,2}):(\d{2})$/.exec(value ?? '');
		if (!match) return null;
		const hour = Number(match[1]);
		const minute = Number(match[2]);
		if (hour > 23 || minute > 59) return null;
		return { hour, minute };
	});

	$effect(() => {
		if (parsed) {
			period = parsed.hour >= 12 ? 'PM' : 'AM';
			hours = parsed.hour % 12 || 12;
			minutes = parsed.minute;
		}
	});

	const label = $derived(
		parsed
			? `${parsed.hour % 12 || 12}:${String(parsed.minute).padStart(2, '0')} ${parsed.hour >= 12 ? 'PM' : 'AM'}`
			: placeholder
	);

	function commit() {
		const hour24 = period === 'AM' ? (hours === 12 ? 0 : hours) : hours === 12 ? 12 : hours + 12;
		value = `${String(hour24).padStart(2, '0')}:${String(minutes).padStart(2, '0')}`;
		onValueChange?.(value);
	}

	function handleHoursInput(e: Event) {
		const input = e.target as HTMLInputElement;
		let val = parseInt(input.value) || 0;
		if (val < 1) val = 1;
		if (val > 12) val = 12;
		hours = val;
		input.value = String(val).padStart(2, '0');
		commit();
	}

	function handleMinutesInput(e: Event) {
		const input = e.target as HTMLInputElement;
		let val = parseInt(input.value) || 0;
		if (val < 0) val = 0;
		if (val > 59) val = 59;
		minutes = val;
		input.value = String(val).padStart(2, '0');
		commit();
	}

	function handlePeriodChange(newPeriod: string | undefined) {
		if (newPeriod === 'AM' || newPeriod === 'PM') {
			period = newPeriod;
			commit();
		}
	}

	function incrementHours() {
		hours = hours >= 12 ? 1 : hours + 1;
		commit();
	}

	function decrementHours() {
		hours = hours <= 1 ? 12 : hours - 1;
		commit();
	}

	function incrementMinutes() {
		if (minutes >= 59) {
			minutes = 0;
			incrementHours();
		} else {
			minutes++;
			commit();
		}
	}

	function decrementMinutes() {
		if (minutes <= 0) {
			minutes = 59;
			decrementHours();
		} else {
			minutes--;
			commit();
		}
	}

	function handleKeyDown(e: KeyboardEvent, field: 'hours' | 'minutes') {
		if (e.key === 'ArrowUp') {
			e.preventDefault();
			if (field === 'hours') incrementHours();
			else incrementMinutes();
		} else if (e.key === 'ArrowDown') {
			e.preventDefault();
			if (field === 'hours') decrementHours();
			else decrementMinutes();
		}
	}
</script>

<Popover.Root bind:open>
	<Popover.Trigger
		{id}
		{disabled}
		class={cn(
			'flex h-9 w-full items-center gap-2 rounded-md border border-input bg-background px-3 text-left text-sm shadow-xs transition-[color,box-shadow] outline-none hover:bg-muted/50 focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50 data-[state=open]:border-ring data-[state=open]:ring-[3px] data-[state=open]:ring-ring/50 dark:bg-input/30',
			className
		)}
	>
		<Clock class="h-4 w-4 shrink-0 text-muted-foreground" />
		<span class={cn('truncate tabular-nums', !parsed && 'text-muted-foreground')}>{label}</span>
	</Popover.Trigger>
	<Popover.Content class="w-auto p-3" align="start">
		<div class="flex items-end gap-2">
			<div class="flex flex-col items-center gap-1">
				<span class="text-xs text-muted-foreground">Hours</span>
				<input
					type="text"
					inputmode="numeric"
					class="h-9 w-10 rounded-md border bg-background text-center text-sm tabular-nums focus:ring-2 focus:ring-ring focus:outline-none"
					value={String(hours).padStart(2, '0')}
					onchange={handleHoursInput}
					onkeydown={(e) => handleKeyDown(e, 'hours')}
					maxlength={2}
				/>
			</div>

			<span class="pb-1 text-sm font-medium text-muted-foreground">:</span>

			<div class="flex flex-col items-center gap-1">
				<span class="text-xs text-muted-foreground">Minutes</span>
				<input
					type="text"
					inputmode="numeric"
					class="h-9 w-10 rounded-md border bg-background text-center text-sm tabular-nums focus:ring-2 focus:ring-ring focus:outline-none"
					value={String(minutes).padStart(2, '0')}
					onchange={handleMinutesInput}
					onkeydown={(e) => handleKeyDown(e, 'minutes')}
					maxlength={2}
				/>
			</div>

			<div class="flex flex-col items-center gap-1">
				<span class="text-xs text-muted-foreground">Period</span>
				<Select.Root type="single" value={period} onValueChange={handlePeriodChange}>
					<Select.Trigger class="h-9 w-[60px] text-sm">
						{period}
					</Select.Trigger>
					<Select.Content>
						<Select.Item value="AM" label="AM">AM</Select.Item>
						<Select.Item value="PM" label="PM">PM</Select.Item>
					</Select.Content>
				</Select.Root>
			</div>
		</div>
	</Popover.Content>
</Popover.Root>
