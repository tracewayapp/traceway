<script lang="ts">
    import { Button } from "$lib/components/ui/button";
    import { DateTimePicker } from "$lib/components/ui/datetime-picker";
    import * as Popover from "$lib/components/ui/popover";
    import { Clock, ChevronDown, ChevronRight, Check } from "@lucide/svelte";
    import { CalendarDate, CalendarDateTime, getLocalTimeZone, today } from "@internationalized/date";

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
        onApply?: (from: { date: CalendarDate; time: string }, to: { date: CalendarDate; time: string }) => void;
    }

    let {
        fromDate = $bindable(today(getLocalTimeZone()).subtract({ days: 7 })),
        toDate = $bindable(today(getLocalTimeZone())),
        fromTime = $bindable('00:00'),
        toTime = $bindable('23:59'),
        onApply
    }: Props = $props();

    // Preset groups
    const presetGroups: PresetGroup[] = [
        {
            presets: [
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
    let showCustom = $state(false);
    let selectedPreset = $state<string | null>('7d');

    // DateTime picker popover states
    let fromPickerOpen = $state(false);
    let toPickerOpen = $state(false);

    // Temporary CalendarDateTime values for custom selection
    let tempFromDateTime = $state<CalendarDateTime>(
        new CalendarDateTime(fromDate.year, fromDate.month, fromDate.day, 0, 0, 0)
    );
    let tempToDateTime = $state<CalendarDateTime>(
        new CalendarDateTime(toDate.year, toDate.month, toDate.day, 23, 59, 59)
    );

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

    function selectPreset(preset: TimeRangePreset) {
        selectedPreset = preset.value;
        showCustom = false;

        // Calculate the date range for this preset
        const now = new Date();
        const fromDateTime = new Date(now.getTime() - preset.minutes * 60 * 1000);

        // Update temp values only - don't close or apply yet
        tempFromDateTime = new CalendarDateTime(
            fromDateTime.getFullYear(),
            fromDateTime.getMonth() + 1,
            fromDateTime.getDate(),
            fromDateTime.getHours(),
            fromDateTime.getMinutes(),
            fromDateTime.getSeconds()
        );
        tempToDateTime = new CalendarDateTime(
            now.getFullYear(),
            now.getMonth() + 1,
            now.getDate(),
            now.getHours(),
            now.getMinutes(),
            now.getSeconds()
        );
    }

    function toggleCustom() {
        showCustom = !showCustom;
        if (showCustom) {
            selectedPreset = null;
        }
    }

    function resetToNow() {
        const now = new Date();
        tempToDateTime = new CalendarDateTime(
            now.getFullYear(), now.getMonth() + 1, now.getDate(),
            now.getHours(), now.getMinutes(), now.getSeconds()
        );
    }

    function handleOpenChange(open: boolean) {
        // Auto-apply when closing (both presets and custom)
        if (!open) {
            fromDate = new CalendarDate(tempFromDateTime.year, tempFromDateTime.month, tempFromDateTime.day);
            toDate = new CalendarDate(tempToDateTime.year, tempToDateTime.month, tempToDateTime.day);
            fromTime = `${String(tempFromDateTime.hour).padStart(2, '0')}:${String(tempFromDateTime.minute).padStart(2, '0')}`;
            toTime = `${String(tempToDateTime.hour).padStart(2, '0')}:${String(tempToDateTime.minute).padStart(2, '0')}`;
            onApply?.({ date: fromDate, time: fromTime }, { date: toDate, time: toTime });

            // Keep showCustom state (don't reset it) so it remembers custom mode
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
    <Popover.Trigger>
        <Button variant="outline" class="h-9 min-w-[340px] justify-between gap-2 font-normal">
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
                        <Button size="sm" onclick={() => isOpen = false}>
                            Apply
                        </Button>
                    </div>
                </div>
            {/if}
        </div>
    </Popover.Content>
</Popover.Root>
