<script lang="ts">
	import * as Popover from '$lib/components/ui/popover';
	import DateTimeCalendar from './datetime-calendar.svelte';
	import { cn } from '$lib/utils';
	import { Calendar as CalendarIcon } from '@lucide/svelte';
	import { CalendarDateTime, getLocalTimeZone, parseDateTime } from '@internationalized/date';

	interface Props {
		// Same wire format as <input type="datetime-local">: yyyy-MM-ddTHH:mm
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
		placeholder = 'Pick a date & time'
	}: Props = $props();

	let open = $state(false);

	const parsed = $derived.by(() => {
		if (!value) return undefined;
		try {
			return parseDateTime(value);
		} catch {
			return undefined;
		}
	});

	const label = $derived(
		parsed
			? parsed.toDate(getLocalTimeZone()).toLocaleString('en-US', {
					month: 'short',
					day: 'numeric',
					year: 'numeric',
					hour: 'numeric',
					minute: '2-digit'
				})
			: placeholder
	);

	function handleChange(dt: CalendarDateTime) {
		const pad = (n: number) => String(n).padStart(2, '0');
		value = `${dt.year}-${pad(dt.month)}-${pad(dt.day)}T${pad(dt.hour)}:${pad(dt.minute)}`;
		onValueChange?.(value);
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
		<CalendarIcon class="h-4 w-4 shrink-0 text-muted-foreground" />
		<span class={cn('truncate tabular-nums', !parsed && 'text-muted-foreground')}>{label}</span>
	</Popover.Trigger>
	<Popover.Content class="w-auto p-0" align="start">
		<DateTimeCalendar value={parsed} onValueChange={handleChange} />
	</Popover.Content>
</Popover.Root>
