<script lang="ts">
	import * as Popover from '$lib/components/ui/popover';
	import { Calendar } from '$lib/components/ui/calendar';
	import { cn } from '$lib/utils';
	import { Calendar as CalendarIcon } from '@lucide/svelte';
	import { getLocalTimeZone, parseDate, type DateValue } from '@internationalized/date';

	interface Props {
		// Same wire format as <input type="date">: yyyy-MM-dd
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
		placeholder = 'Pick a date'
	}: Props = $props();

	let open = $state(false);

	const parsed = $derived.by(() => {
		if (!value) return undefined;
		try {
			return parseDate(value);
		} catch {
			return undefined;
		}
	});

	const label = $derived(
		parsed
			? parsed.toDate(getLocalTimeZone()).toLocaleDateString('en-US', {
					month: 'short',
					day: 'numeric',
					year: 'numeric'
				})
			: placeholder
	);

	function handleSelect(date: DateValue | undefined) {
		if (!date) return;
		const pad = (n: number) => String(n).padStart(2, '0');
		value = `${date.year}-${pad(date.month)}-${pad(date.day)}`;
		onValueChange?.(value);
		open = false;
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
		<Calendar type="single" value={parsed} onValueChange={handleSelect} />
	</Popover.Content>
</Popover.Root>
