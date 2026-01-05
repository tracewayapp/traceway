<script lang="ts">
    import * as Popover from "$lib/components/ui/popover";
    import { Calendar } from "$lib/components/ui/calendar";
    import { Button } from "$lib/components/ui/button";
    import { Input } from "$lib/components/ui/input";
    import { Label } from "$lib/components/ui/label";
    import { CalendarDays, ChevronDown } from "@lucide/svelte";
    import { CalendarDate, getLocalTimeZone, today } from "@internationalized/date";
    import type { DateValue } from "@internationalized/date";

    type Props = {
        fromDate: string;
        toDate: string;
        onApply: (from: string, to: string) => void;
        class?: string;
    };

    let { fromDate, toDate, onApply, class: className = '' }: Props = $props();

    let open = $state(false);
    let fromCalendarOpen = $state(false);
    let toCalendarOpen = $state(false);

    // Internal state for dates and times
    let tempFromDate = $state<DateValue | undefined>(undefined);
    let tempFromTime = $state('00:00');
    let tempToDate = $state<DateValue | undefined>(undefined);
    let tempToTime = $state('23:59');

    // Parse the ISO string into CalendarDate and time string
    function parseDateTime(dateTimeStr: string): { date: DateValue | undefined; time: string } {
        if (!dateTimeStr) return { date: undefined, time: '00:00' };
        try {
            const d = new Date(dateTimeStr);
            const calDate = new CalendarDate(d.getFullYear(), d.getMonth() + 1, d.getDate());
            const hours = d.getHours().toString().padStart(2, '0');
            const minutes = d.getMinutes().toString().padStart(2, '0');
            return { date: calDate, time: `${hours}:${minutes}` };
        } catch {
            return { date: undefined, time: '00:00' };
        }
    }

    // Combine CalendarDate and time string into ISO string
    function combineDateTime(calDate: DateValue | undefined, time: string): string {
        if (!calDate) return '';
        const [hours, minutes] = time.split(':').map(Number);
        const jsDate = calDate.toDate(getLocalTimeZone());
        jsDate.setHours(hours || 0, minutes || 0, 0, 0);
        return jsDate.toISOString().slice(0, 16);
    }

    // Sync temp values when popover opens or props change
    $effect(() => {
        if (open) {
            const from = parseDateTime(fromDate);
            const to = parseDateTime(toDate);
            tempFromDate = from.date;
            tempFromTime = from.time;
            tempToDate = to.date;
            tempToTime = to.time;
        }
    });

    function handleApply() {
        const from = combineDateTime(tempFromDate, tempFromTime);
        const to = combineDateTime(tempToDate, tempToTime);
        if (from && to) {
            onApply(from, to);
        }
        open = false;
    }

    function handleCancel() {
        open = false;
    }

    function formatDisplayDate(dateStr: string): string {
        if (!dateStr) return '';
        try {
            const date = new Date(dateStr);
            return date.toLocaleDateString('en-US', {
                month: 'short',
                day: 'numeric',
                hour: '2-digit',
                minute: '2-digit'
            });
        } catch {
            return dateStr;
        }
    }

    function formatCalendarDate(date: DateValue | undefined): string {
        if (!date) return 'Select date';
        return date.toDate(getLocalTimeZone()).toLocaleDateString('en-US', {
            month: 'short',
            day: 'numeric',
            year: 'numeric'
        });
    }

    const displayText = $derived(() => {
        if (!fromDate || !toDate) return 'Select date range';
        return `${formatDisplayDate(fromDate)} – ${formatDisplayDate(toDate)}`;
    });
</script>

<Popover.Root bind:open>
    <Popover.Trigger>
        {#snippet child({ props })}
            <Button
                variant="outline"
                class="h-9 min-w-[280px] justify-start text-left font-normal {className}"
                {...props}
            >
                <CalendarDays class="mr-2 h-4 w-4 opacity-70" />
                <span class="truncate">{displayText()}</span>
            </Button>
        {/snippet}
    </Popover.Trigger>
    <Popover.Content class="w-auto p-0" align="start">
        <div class="p-4 space-y-4">
            <div class="grid grid-cols-2 gap-6">
                <!-- From Date/Time -->
                <div class="space-y-3">
                    <Label class="text-xs font-medium text-muted-foreground">From</Label>

                    <!-- Date Picker -->
                    <Popover.Root bind:open={fromCalendarOpen}>
                        <Popover.Trigger>
                            {#snippet child({ props })}
                                <Button
                                    variant="outline"
                                    class="w-full justify-between font-normal text-sm h-9"
                                    {...props}
                                >
                                    {formatCalendarDate(tempFromDate)}
                                    <ChevronDown class="h-4 w-4 opacity-50" />
                                </Button>
                            {/snippet}
                        </Popover.Trigger>
                        <Popover.Content class="w-auto p-0" align="start">
                            <Calendar
                                type="single"
                                bind:value={tempFromDate}
                                onValueChange={() => {
                                    fromCalendarOpen = false;
                                }}
                                captionLayout="dropdown"
                            />
                        </Popover.Content>
                    </Popover.Root>

                    <!-- Time Input -->
                    <Input
                        type="time"
                        class="h-9 text-sm bg-background appearance-none [&::-webkit-calendar-picker-indicator]:hidden [&::-webkit-calendar-picker-indicator]:appearance-none"
                        bind:value={tempFromTime}
                    />
                </div>

                <!-- To Date/Time -->
                <div class="space-y-3">
                    <Label class="text-xs font-medium text-muted-foreground">To</Label>

                    <!-- Date Picker -->
                    <Popover.Root bind:open={toCalendarOpen}>
                        <Popover.Trigger>
                            {#snippet child({ props })}
                                <Button
                                    variant="outline"
                                    class="w-full justify-between font-normal text-sm h-9"
                                    {...props}
                                >
                                    {formatCalendarDate(tempToDate)}
                                    <ChevronDown class="h-4 w-4 opacity-50" />
                                </Button>
                            {/snippet}
                        </Popover.Trigger>
                        <Popover.Content class="w-auto p-0" align="start">
                            <Calendar
                                type="single"
                                bind:value={tempToDate}
                                onValueChange={() => {
                                    toCalendarOpen = false;
                                }}
                                captionLayout="dropdown"
                            />
                        </Popover.Content>
                    </Popover.Root>

                    <!-- Time Input -->
                    <Input
                        type="time"
                        class="h-9 text-sm bg-background appearance-none [&::-webkit-calendar-picker-indicator]:hidden [&::-webkit-calendar-picker-indicator]:appearance-none"
                        bind:value={tempToTime}
                    />
                </div>
            </div>
            <div class="flex items-center justify-end gap-2 pt-2 border-t">
                <Button variant="ghost" size="sm" onclick={handleCancel}>
                    Cancel
                </Button>
                <Button size="sm" onclick={handleApply}>
                    Apply
                </Button>
            </div>
        </div>
    </Popover.Content>
</Popover.Root>
