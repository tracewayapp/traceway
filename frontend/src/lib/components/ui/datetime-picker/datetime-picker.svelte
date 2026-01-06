<script lang="ts">
    import { Calendar } from "$lib/components/ui/calendar";
    import * as Select from "$lib/components/ui/select";
    import { cn } from "$lib/utils";
    import { CalendarDate, CalendarDateTime, getLocalTimeZone, today, type DateValue } from "@internationalized/date";

    type Period = 'AM' | 'PM';

    interface Props {
        value?: CalendarDateTime | CalendarDate;
        onValueChange?: (value: CalendarDateTime) => void;
        class?: string;
    }

    let {
        value = $bindable(),
        onValueChange,
        class: className
    }: Props = $props();

    // Time state (12-hour format for display)
    let hours = $state(12);
    let minutes = $state(0);
    let seconds = $state(0);
    let period = $state<Period>('AM');

    // Initialize from value
    $effect(() => {
        if (value && 'hour' in value) {
            const hour24 = value.hour;
            period = hour24 >= 12 ? 'PM' : 'AM';
            hours = hour24 % 12 || 12;
            minutes = value.minute;
            seconds = value.second;
        }
    });

    // Derived 24-hour value
    const hour24 = $derived(() => {
        if (period === 'AM') {
            return hours === 12 ? 0 : hours;
        } else {
            return hours === 12 ? 12 : hours + 12;
        }
    });

    // Handle calendar date change
    function handleDateChange(newDate: DateValue | undefined) {
        if (!newDate) return;

        const dateTime = new CalendarDateTime(
            newDate.year,
            newDate.month,
            newDate.day,
            hour24(),
            minutes,
            seconds
        );

        value = dateTime;
        onValueChange?.(dateTime);
    }

    // Handle time changes
    function updateDateTime() {
        if (!value) {
            const now = today(getLocalTimeZone());
            value = new CalendarDateTime(now.year, now.month, now.day, hour24(), minutes, seconds);
        } else {
            const dateTime = new CalendarDateTime(
                value.year,
                value.month,
                value.day,
                hour24(),
                minutes,
                seconds
            );
            value = dateTime;
            onValueChange?.(dateTime);
        }
    }

    // Input handlers with validation
    function handleHoursInput(e: Event) {
        const input = e.target as HTMLInputElement;
        let val = parseInt(input.value) || 0;
        if (val < 1) val = 1;
        if (val > 12) val = 12;
        hours = val;
        input.value = String(val).padStart(2, '0');
        updateDateTime();
    }

    function handleMinutesInput(e: Event) {
        const input = e.target as HTMLInputElement;
        let val = parseInt(input.value) || 0;
        if (val < 0) val = 0;
        if (val > 59) val = 59;
        minutes = val;
        input.value = String(val).padStart(2, '0');
        updateDateTime();
    }

    function handleSecondsInput(e: Event) {
        const input = e.target as HTMLInputElement;
        let val = parseInt(input.value) || 0;
        if (val < 0) val = 0;
        if (val > 59) val = 59;
        seconds = val;
        input.value = String(val).padStart(2, '0');
        updateDateTime();
    }

    function handlePeriodChange(newPeriod: string | undefined) {
        if (newPeriod === 'AM' || newPeriod === 'PM') {
            period = newPeriod;
            updateDateTime();
        }
    }

    // Increment/decrement helpers
    function incrementHours() {
        hours = hours >= 12 ? 1 : hours + 1;
        updateDateTime();
    }

    function decrementHours() {
        hours = hours <= 1 ? 12 : hours - 1;
        updateDateTime();
    }

    function incrementMinutes() {
        if (minutes >= 59) {
            minutes = 0;
            incrementHours();
        } else {
            minutes++;
        }
        updateDateTime();
    }

    function decrementMinutes() {
        if (minutes <= 0) {
            minutes = 59;
            decrementHours();
        } else {
            minutes--;
        }
        updateDateTime();
    }

    function incrementSeconds() {
        if (seconds >= 59) {
            seconds = 0;
            incrementMinutes();
        } else {
            seconds++;
        }
        updateDateTime();
    }

    function decrementSeconds() {
        if (seconds <= 0) {
            seconds = 59;
            decrementMinutes();
        } else {
            seconds--;
        }
        updateDateTime();
    }

    // Handle keyboard navigation
    function handleKeyDown(e: KeyboardEvent, field: 'hours' | 'minutes' | 'seconds') {
        if (e.key === 'ArrowUp') {
            e.preventDefault();
            if (field === 'hours') incrementHours();
            else if (field === 'minutes') incrementMinutes();
            else incrementSeconds();
        } else if (e.key === 'ArrowDown') {
            e.preventDefault();
            if (field === 'hours') decrementHours();
            else if (field === 'minutes') decrementMinutes();
            else decrementSeconds();
        }
    }

    // Get calendar-compatible date value
    const calendarValue = $derived(() => {
        if (!value) return undefined;
        return new CalendarDate(value.year, value.month, value.day);
    });
</script>

<div class={cn("flex flex-col", className)}>
    <!-- Calendar -->
    <Calendar
        type="single"
        value={calendarValue()}
        onValueChange={handleDateChange}
    />

    <!-- Time Picker -->
    <div class="border-t px-3 py-3">
        <div class="flex items-end gap-2">
            <!-- Hours -->
            <div class="flex flex-col items-center gap-1">
                <span class="text-xs text-muted-foreground">Hours</span>
                <input
                    type="text"
                    inputmode="numeric"
                    class="w-10 h-9 text-center text-sm border rounded-md bg-background focus:outline-none focus:ring-2 focus:ring-ring tabular-nums"
                    value={String(hours).padStart(2, '0')}
                    onchange={handleHoursInput}
                    onkeydown={(e) => handleKeyDown(e, 'hours')}
                    maxlength={2}
                />
            </div>

            <span class="text-sm font-medium text-muted-foreground pb-1">:</span>

            <!-- Minutes -->
            <div class="flex flex-col items-center gap-1">
                <span class="text-xs text-muted-foreground">Minutes</span>
                <input
                    type="text"
                    inputmode="numeric"
                    class="w-10 h-9 text-center text-sm border rounded-md bg-background focus:outline-none focus:ring-2 focus:ring-ring tabular-nums"
                    value={String(minutes).padStart(2, '0')}
                    onchange={handleMinutesInput}
                    onkeydown={(e) => handleKeyDown(e, 'minutes')}
                    maxlength={2}
                />
            </div>

            <span class="text-sm font-medium text-muted-foreground pb-1">:</span>

            <!-- Seconds -->
            <div class="flex flex-col items-center gap-1">
                <span class="text-xs text-muted-foreground">Seconds</span>
                <input
                    type="text"
                    inputmode="numeric"
                    class="w-10 h-9 text-center text-sm border rounded-md bg-background focus:outline-none focus:ring-2 focus:ring-ring tabular-nums"
                    value={String(seconds).padStart(2, '0')}
                    onchange={handleSecondsInput}
                    onkeydown={(e) => handleKeyDown(e, 'seconds')}
                    maxlength={2}
                />
            </div>

            <!-- Period (AM/PM) -->
            <div class="flex flex-col items-center gap-1">
                <span class="text-xs text-muted-foreground">Period</span>
                <Select.Root
                    type="single"
                    value={period}
                    onValueChange={handlePeriodChange}
                >
                    <Select.Trigger class="w-[60px] h-9 text-sm">
                        {period}
                    </Select.Trigger>
                    <Select.Content>
                        <Select.Item value="AM" label="AM">AM</Select.Item>
                        <Select.Item value="PM" label="PM">PM</Select.Item>
                    </Select.Content>
                </Select.Root>
            </div>
        </div>
    </div>
</div>
