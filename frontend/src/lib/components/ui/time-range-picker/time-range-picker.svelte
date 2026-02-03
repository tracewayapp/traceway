<script lang="ts">
    import { Button } from "$lib/components/ui/button";
    import { DateTimePicker } from "$lib/components/ui/datetime-picker";
    import * as Popover from "$lib/components/ui/popover";
    import { Clock, ChevronDown, ChevronRight, Check } from "@lucide/svelte";
    import { CalendarDate, CalendarDateTime, getLocalTimeZone, today } from "@internationalized/date";
    import { getNow, luxonToCalendarDateTime } from "$lib/utils/formatters";
    import { getTimezone } from "$lib/state/timezone.svelte";

    type TimeRangePreset = {
        value: string;
        label: string;
        minutes: number;
    };

    type PresetGroup = {
        presets: TimeRangePreset[];
    };

    interface Props {
        fromDate?: CalendarDate;
        toDate?: CalendarDate;
        fromTime?: string;
        toTime?: string;
        preset?: string | null;
        timezone?: string;
        onApply?: (from: { date: CalendarDate; time: string }, to: { date: CalendarDate; time: string }, preset: string | null) => void;
    }

    let {
        fromDate = $bindable(today(getLocalTimeZone()).subtract({ days: 7 })),
        toDate = $bindable(today(getLocalTimeZone())),
        fromTime = $bindable('00:00'),
        toTime = $bindable('23:59'),
        preset = $bindable<string | null>('24h'),
        timezone,
        onApply
    }: Props = $props();

    const tz = $derived(timezone ?? getTimezone());

    // Preset groups
    const presetGroups: PresetGroup[] = [
        {
            presets: [
                { value: '5m', label: '5 minutes', minutes: 5 },
                { value: '30m', label: '30 minutes', minutes: 30 },
                { value: '60m', label: '60 minutes', minutes: 60 },
            ]
        },
        {
            presets: [
                { value: '3h', label: '3 hours', minutes: 180 },
                { value: '6h', label: '6 hours', minutes: 360 },
                { value: '12h', label: '12 hours', minutes: 720 },
                { value: '24h', label: '24 hours', minutes: 1440 },
            ]
        },
        {
            presets: [
                { value: '3d', label: '3 days', minutes: 4320 },
                { value: '7d', label: '7 days', minutes: 10080 },
            ]
        },
        {
            presets: [
                { value: '1M', label: '1 month', minutes: 43200 },
                { value: '3M', label: '3 months', minutes: 129600 },
            ]
        }
    ];

    let isOpen = $state(false);
    let showCustom = $state(preset === null);
    let selectedPreset = $state<string | null>(preset);

    // Flag to ignore external change detection when we're applying our own changes
    let isApplyingInternally = $state(false);

    // Sync internal selectedPreset with external preset prop
    $effect(() => {
        if (preset !== selectedPreset) {
            selectedPreset = preset;
            showCustom = preset === null;

            // If preset is set, calculate the time range
            if (preset) {
                const presetDef = presetGroups.flatMap(g => g.presets).find(p => p.value === preset);
                if (presetDef) {
                    const now = getNow(tz);
                    const fromDt = now.minus({ minutes: presetDef.minutes });
                    const fromParts = luxonToCalendarDateTime(fromDt);
                    const toParts = luxonToCalendarDateTime(now);

                    tempFromDateTime = new CalendarDateTime(
                        fromParts.year,
                        fromParts.month,
                        fromParts.day,
                        fromParts.hour,
                        fromParts.minute,
                        fromParts.second
                    );
                    tempToDateTime = new CalendarDateTime(
                        toParts.year,
                        toParts.month,
                        toParts.day,
                        toParts.hour,
                        toParts.minute,
                        toParts.second
                    );
                }
            }
        }
    });

    // Track last known bound values to detect external changes
    let lastFromDate = $state(fromDate);
    let lastToDate = $state(toDate);
    let lastFromTime = $state(fromTime);
    let lastToTime = $state(toTime);

    // Detect external value changes and switch to custom mode
    $effect(() => {
        const fromDateChanged = fromDate.year !== lastFromDate.year ||
            fromDate.month !== lastFromDate.month ||
            fromDate.day !== lastFromDate.day;
        const toDateChanged = toDate.year !== lastToDate.year ||
            toDate.month !== lastToDate.month ||
            toDate.day !== lastToDate.day;
        const fromTimeChanged = fromTime !== lastFromTime;
        const toTimeChanged = toTime !== lastToTime;

        if (fromDateChanged || toDateChanged || fromTimeChanged || toTimeChanged) {
            // Update last known values
            lastFromDate = fromDate;
            lastToDate = toDate;
            lastFromTime = fromTime;
            lastToTime = toTime;

            // If picker is not open AND we're not applying internally, this was an external change
            if (!isOpen && !isApplyingInternally) {
                showCustom = true;
                selectedPreset = null;

                // Sync temp values with the new bound values
                const [fromHour, fromMinute] = fromTime.split(':').map(Number);
                const [toHour, toMinute] = toTime.split(':').map(Number);
                tempFromDateTime = new CalendarDateTime(
                    fromDate.year, fromDate.month, fromDate.day,
                    fromHour || 0, fromMinute || 0, 0
                );
                tempToDateTime = new CalendarDateTime(
                    toDate.year, toDate.month, toDate.day,
                    toHour || 23, toMinute || 59, 0
                );
            }
        }
    });

    // DateTime picker popover states
    let fromPickerOpen = $state(false);
    let toPickerOpen = $state(false);

    // Temporary CalendarDateTime values for custom selection
    // Parse initial time from props instead of using hardcoded values
    function parseInitialTime(timeStr: string, fallbackHour: number, fallbackMinute: number): { hour: number; minute: number } {
        if (!timeStr) return { hour: fallbackHour, minute: fallbackMinute };
        const [hour, minute] = timeStr.split(':').map(Number);
        return { hour: hour ?? fallbackHour, minute: minute ?? fallbackMinute };
    }

    const initialFromTimeParts = parseInitialTime(fromTime, 0, 0);
    const initialToTimeParts = parseInitialTime(toTime, 23, 59);

    let tempFromDateTime = $state<CalendarDateTime>(
        new CalendarDateTime(fromDate.year, fromDate.month, fromDate.day, initialFromTimeParts.hour, initialFromTimeParts.minute, 0)
    );
    let tempToDateTime = $state<CalendarDateTime>(
        new CalendarDateTime(toDate.year, toDate.month, toDate.day, initialToTimeParts.hour, initialToTimeParts.minute, 59)
    );

    // Track initial values when popover opens to detect changes
    let initialFromDateTime = $state<CalendarDateTime | null>(null);
    let initialToDateTime = $state<CalendarDateTime | null>(null);

    // Helper to compare two CalendarDateTime values
    function dateTimesEqual(a: CalendarDateTime, b: CalendarDateTime): boolean {
        return a.year === b.year &&
               a.month === b.month &&
               a.day === b.day &&
               a.hour === b.hour &&
               a.minute === b.minute;
    }

    // Get display label for trigger button
    const displayLabel = $derived(() => {
        if (selectedPreset && !showCustom) {
            const preset = presetGroups.flatMap(g => g.presets).find(p => p.value === selectedPreset);
            return preset ? `Last ${preset.label}` : 'Select time range';
        }
        // For custom selection, show date and time range (no prefix)
        if (tempFromDateTime && tempToDateTime) {
            const fromDateStr = formatDateShort(tempFromDateTime);
            const fromTimeStr = formatTime12h(tempFromDateTime);
            const toDateStr = formatDateShort(tempToDateTime);
            const toTimeStr = formatTime12h(tempToDateTime);
            return `${fromDateStr} ${fromTimeStr} – ${toDateStr} ${toTimeStr}`;
        }
        return 'Select time range';
    });

    function formatDateShort(dt: CalendarDateTime): string {
        return new CalendarDate(dt.year, dt.month, dt.day)
            .toDate(getLocalTimeZone())
            .toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
    }

    // YY/MM/DD format for compact display
    function formatDateCompact(dt: CalendarDateTime): string {
        const year = String(dt.year).slice(-2);
        const month = String(dt.month).padStart(2, '0');
        const day = String(dt.day).padStart(2, '0');
        return `${year}/${month}/${day}`;
    }

    // Format time as 12h with AM/PM
    function formatTime12h(dt: CalendarDateTime): string {
        const hours = dt.hour;
        const minutes = dt.minute;
        const period = hours >= 12 ? 'PM' : 'AM';
        const hours12 = hours % 12 || 12;
        return `${hours12}:${String(minutes).padStart(2, '0')} ${period}`;
    }

    function selectPreset(presetDef: TimeRangePreset) {
        selectedPreset = presetDef.value;
        preset = presetDef.value; // Sync with external prop
        showCustom = false;

        // Calculate the date range for this preset using Luxon
        const now = getNow(tz);
        const fromDt = now.minus({ minutes: presetDef.minutes });
        const fromParts = luxonToCalendarDateTime(fromDt);
        const toParts = luxonToCalendarDateTime(now);

        // Update temp values only - don't close or apply yet
        tempFromDateTime = new CalendarDateTime(
            fromParts.year,
            fromParts.month,
            fromParts.day,
            fromParts.hour,
            fromParts.minute,
            fromParts.second
        );
        tempToDateTime = new CalendarDateTime(
            toParts.year,
            toParts.month,
            toParts.day,
            toParts.hour,
            toParts.minute,
            toParts.second
        );
    }

    function toggleCustom() {
        showCustom = !showCustom;
        if (showCustom) {
            selectedPreset = null;
            preset = null; // Sync with external prop
        }
    }

    function resetToNow() {
        const now = getNow(tz);
        const nowParts = luxonToCalendarDateTime(now);
        tempToDateTime = new CalendarDateTime(
            nowParts.year, nowParts.month, nowParts.day,
            nowParts.hour, nowParts.minute, nowParts.second
        );
    }

    function applyAndClose() {
        isApplyingInternally = true;

        fromDate = new CalendarDate(tempFromDateTime.year, tempFromDateTime.month, tempFromDateTime.day);
        toDate = new CalendarDate(tempToDateTime.year, tempToDateTime.month, tempToDateTime.day);
        fromTime = `${String(tempFromDateTime.hour).padStart(2, '0')}:${String(tempFromDateTime.minute).padStart(2, '0')}`;
        toTime = `${String(tempToDateTime.hour).padStart(2, '0')}:${String(tempToDateTime.minute).padStart(2, '0')}`;

        const currentPreset = showCustom ? null : selectedPreset;
        preset = currentPreset;
        onApply?.({ date: fromDate, time: fromTime }, { date: toDate, time: toTime }, currentPreset);

        setTimeout(() => { isApplyingInternally = false; }, 0);

        fromPickerOpen = false;
        toPickerOpen = false;
        isOpen = false;
    }

    function handleOpenChange(open: boolean) {
        if (open) {
            // Capture initial values when opening
            initialFromDateTime = new CalendarDateTime(
                tempFromDateTime.year, tempFromDateTime.month, tempFromDateTime.day,
                tempFromDateTime.hour, tempFromDateTime.minute, tempFromDateTime.second
            );
            initialToDateTime = new CalendarDateTime(
                tempToDateTime.year, tempToDateTime.month, tempToDateTime.day,
                tempToDateTime.hour, tempToDateTime.minute, tempToDateTime.second
            );
        } else {
            // Skip if already applied by applyAndClose (Apply button was clicked)
            if (isApplyingInternally) {
                isOpen = open;
                return;
            }

            // Apply if closed by clicking outside and values have changed
            const hasChanged = !initialFromDateTime || !initialToDateTime ||
                !dateTimesEqual(tempFromDateTime, initialFromDateTime) ||
                !dateTimesEqual(tempToDateTime, initialToDateTime);

            if (hasChanged) {
                applyAndClose();
                return; // applyAndClose handles setting isOpen
            }

            // No changes - just close
            fromPickerOpen = false;
            toPickerOpen = false;
        }
        isOpen = open;
    }

    function handleFromDateTimeChange(dt: CalendarDateTime) {
        tempFromDateTime = dt;
    }

    function handleToDateTimeChange(dt: CalendarDateTime) {
        tempToDateTime = dt;
    }
</script>

<Popover.Root bind:open={isOpen} onOpenChange={handleOpenChange}>
    <Popover.Trigger class="w-full">
        <Button variant="outline" class="h-9 justify-between gap-2 font-normal w-full">
            <span class="flex items-center gap-2">
                <Clock class="h-4 w-4 text-muted-foreground" />
                <span class="truncate">{displayLabel()}</span>
            </span>
            <ChevronDown class="h-4 w-4 text-muted-foreground" />
        </Button>
    </Popover.Trigger>
    <Popover.Content class="w-auto p-0" align="start">
        <div class="flex">
            <!-- Presets Column -->
            <div class="w-[140px] border-r py-2">
                {#each presetGroups as group, groupIndex}
                    {#if groupIndex > 0}
                        <div class="my-1 border-t mx-2"></div>
                    {/if}
                    {#each group.presets as preset}
                        <button
                            class="w-full px-3 py-1.5 text-left text-sm hover:bg-muted/50 flex items-center justify-between transition-colors {selectedPreset === preset.value && !showCustom ? 'bg-muted' : ''}"
                            onclick={() => selectPreset(preset)}
                        >
                            <span>{preset.label}</span>
                            {#if selectedPreset === preset.value && !showCustom}
                                <Check class="h-4 w-4 text-primary" />
                            {/if}
                        </button>
                    {/each}
                {/each}
                <div class="my-1 border-t mx-2"></div>
                <button
                    class="w-full px-3 py-1.5 text-left text-sm hover:bg-muted/50 flex items-center justify-between transition-colors {showCustom ? 'bg-muted' : ''}"
                    onclick={toggleCustom}
                >
                    <span>Set custom</span>
                    <ChevronRight class="h-4 w-4 text-muted-foreground" />
                </button>
            </div>

            <!-- Custom Section -->
            {#if showCustom}
                <div class="p-5 min-w-[380px]">
                    <div class="flex items-center justify-between mb-3">
                        <span class="text-xs font-medium text-muted-foreground uppercase tracking-wide">Custom</span>
                        <button
                            class="text-xs text-primary hover:underline"
                            onclick={resetToNow}
                        >
                            Reset to now
                        </button>
                    </div>

                    <!-- Compact Date/Time Inputs Row -->
                    <div class="flex items-center gap-2 mb-4">
                        <!-- From DateTime -->
                        <Popover.Root bind:open={fromPickerOpen}>
                            <Popover.Trigger>
                                <button
                                    class="h-9 px-3 text-sm border rounded-md text-left hover:bg-muted/50 transition-colors flex items-center gap-2 {fromPickerOpen ? 'ring-2 ring-ring' : ''}"
                                >
                                    <span class="tabular-nums">{formatDateCompact(tempFromDateTime)}</span>
                                    <span class="text-muted-foreground tabular-nums">{formatTime12h(tempFromDateTime)}</span>
                                </button>
                            </Popover.Trigger>
                            <Popover.Content class="w-auto p-0" align="start">
                                <DateTimePicker
                                    value={tempFromDateTime}
                                    onValueChange={handleFromDateTimeChange}
                                />
                            </Popover.Content>
                        </Popover.Root>

                        <span class="text-muted-foreground text-xs">–</span>

                        <!-- To DateTime -->
                        <Popover.Root bind:open={toPickerOpen}>
                            <Popover.Trigger>
                                <button
                                    class="h-9 px-3 text-sm border rounded-md text-left hover:bg-muted/50 transition-colors flex items-center gap-2 {toPickerOpen ? 'ring-2 ring-ring' : ''}"
                                >
                                    <span class="tabular-nums">{formatDateCompact(tempToDateTime)}</span>
                                    <span class="text-muted-foreground tabular-nums">{formatTime12h(tempToDateTime)}</span>
                                </button>
                            </Popover.Trigger>
                            <Popover.Content class="w-auto p-0" align="start">
                                <DateTimePicker
                                    value={tempToDateTime}
                                    onValueChange={handleToDateTimeChange}
                                />
                            </Popover.Content>
                        </Popover.Root>
                    </div>

                    <!-- Apply button -->
                    <div class="flex justify-end pt-2">
                        <Button size="sm" onclick={applyAndClose}>
                            Apply
                        </Button>
                    </div>
                </div>
            {/if}
        </div>
    </Popover.Content>
</Popover.Root>
