<script lang="ts">
    import { onMount, onDestroy } from 'svelte';
    import { api } from '$lib/api';
    import { formatDuration, formatRelativeTime, toUTCISO, calendarDateTimeToLuxon } from '$lib/utils/formatters';
    import { getTimezone } from '$lib/state/timezone.svelte';
    import * as Table from "$lib/components/ui/table";
    import { LoadingCircle } from "$lib/components/ui/loading-circle";
    import { TracewayTableHeader } from "$lib/components/ui/traceway-table-header";
    import { TableEmptyState } from "$lib/components/ui/table-empty-state";
    import { PaginationFooter } from "$lib/components/ui/pagination-footer";
    import { TimeRangePicker } from "$lib/components/ui/time-range-picker";
    import { SearchBar } from "$lib/components/ui/search-bar";
    import * as AlertDialog from '$lib/components/ui/alert-dialog';
    import * as Tooltip from '$lib/components/ui/tooltip';
    import { Button } from '$lib/components/ui/button';
    import { Input } from '$lib/components/ui/input';
    import Plus from '@lucide/svelte/icons/plus';
    import X from '@lucide/svelte/icons/x';
    import Info from '@lucide/svelte/icons/info';
    import { CalendarDate } from "@internationalized/date";
    import { browser } from '$app/environment';
    import { goto } from '$app/navigation';
    import { projectsState } from '$lib/state/projects.svelte';
    import { createRowClickHandler } from '$lib/utils/navigation';
    import { resolve } from '$app/paths';
    import PageHeader from '$lib/components/issues/page-header.svelte';
    import {
        getTimeRangeFromPreset,
        dateToCalendarDate,
        dateToTimeString,
        parseTimeRangeFromUrl,
        getResolvedTimeRange,
        updateUrl
    } from '$lib/utils/url-params';
    import {
        getSortState,
        setSortState,
        handleSortClick,
        type SortDirection
    } from '$lib/utils/sort-storage';

    const timezone = $derived(getTimezone());

    type Session = {
        id: string;
        startedAt: string;
        endedAt?: string | null;
        duration: number;
        clientIP: string;
        appVersion: string;
        serverName: string;
    };

    type SortField = 'started_at' | 'duration';

    let sessions = $state<Session[]>([]);
    let loading = $state(true);
    let error = $state('');

    let page = $state(1);
    let pageSize = $state(50);
    let total = $state(0);
    let totalPages = $state(0);

    type AttributeFilter = { key: string; value: string };

    function parseAttributeFilter(input: string): AttributeFilter | null {
        const idx = input.indexOf('=');
        if (idx <= 0) return null;
        const key = input.slice(0, idx).trim();
        const value = input.slice(idx + 1);
        if (!key) return null;
        return { key, value };
    }

    function parseSessionsUrlParams() {
        const timeParams = parseTimeRangeFromUrl(timezone);
        if (!browser) return { ...timeParams, search: '', attributeFilters: [] as AttributeFilter[] };
        const params = new URLSearchParams(window.location.search);
        const attrs: AttributeFilter[] = [];
        for (const raw of params.getAll('attr')) {
            const parsed = parseAttributeFilter(raw);
            if (parsed) attrs.push(parsed);
        }
        return {
            ...timeParams,
            search: params.get('search') ?? '',
            attributeFilters: attrs
        };
    }

    const initialUrlParams = parseSessionsUrlParams();
    const initialRange = getResolvedTimeRange(initialUrlParams, timezone);

    let selectedPreset = $state<string | null>(initialUrlParams.preset);
    let fromDate = $state<CalendarDate>(dateToCalendarDate(initialRange.from, timezone));
    let toDate = $state<CalendarDate>(dateToCalendarDate(initialRange.to, timezone));
    let fromTime = $state(dateToTimeString(initialRange.from, timezone));
    let toTime = $state(dateToTimeString(initialRange.to, timezone));
    let searchQuery = $state(initialUrlParams.search);
    let attributeFilters = $state<AttributeFilter[]>(initialUrlParams.attributeFilters);

    let addFilterOpen = $state(false);
    let dialogKey = $state('');
    let dialogValue = $state('');
    let dialogError = $state('');

    function openAddFilterDialog() {
        dialogKey = '';
        dialogValue = '';
        dialogError = '';
        addFilterOpen = true;
    }

    function submitDialogFilter() {
        const key = dialogKey.trim();
        if (!key) {
            dialogError = 'Attribute key is required';
            return;
        }
        const value = dialogValue;
        const dup = attributeFilters.some((f) => f.key === key && f.value === value);
        if (!dup) {
            attributeFilters = [...attributeFilters, { key, value }];
        }
        addFilterOpen = false;
        page = 1;
        loadData(true);
    }

    function handleDialogKeydown(e: KeyboardEvent) {
        if (e.key === 'Enter') {
            e.preventDefault();
            submitDialogFilter();
        }
    }

    function removeAttributeFilter(index: number) {
        attributeFilters = attributeFilters.filter((_, i) => i !== index);
        page = 1;
        loadData(true);
    }

    function updateTimeRangeUrl(pushToHistory = true) {
        if (!browser) return;
        const params = new URLSearchParams();
        if (selectedPreset) {
            params.set('preset', selectedPreset);
        } else {
            params.set('from', getFromDateTimeUTC());
            params.set('to', getToDateTimeUTC());
        }
        if (searchQuery.trim()) params.set('search', searchQuery.trim());
        for (const f of attributeFilters) {
            params.append('attr', `${f.key}=${f.value}`);
        }
        const newUrl = `${window.location.pathname}?${params.toString()}`;
        // eslint-disable-next-line svelte/no-navigation-without-resolve
        goto(newUrl, { replaceState: !pushToHistory, noScroll: true, keepFocus: true });
    }

    function handlePopState() {
        const urlParams = parseSessionsUrlParams();
        const range = getResolvedTimeRange(urlParams, timezone);
        selectedPreset = urlParams.preset;
        fromDate = dateToCalendarDate(range.from, timezone);
        fromTime = dateToTimeString(range.from, timezone);
        toDate = dateToCalendarDate(range.to, timezone);
        toTime = dateToTimeString(range.to, timezone);
        searchQuery = urlParams.search;
        attributeFilters = urlParams.attributeFilters;
        page = 1;
        loadData(false);
    }

    function handleSearch() {
        page = 1;
        loadData(true);
    }

    const SORT_STORAGE_KEY = 'sessions';
    const initialSort = getSortState(SORT_STORAGE_KEY, { field: 'started_at', direction: 'desc' });
    let orderBy = $state<SortField>(initialSort.field as SortField);
    let sortDirection = $state<SortDirection>(initialSort.direction);

    function getFromDateTimeUTC(): string {
        const [hour, minute] = (fromTime || '00:00').split(':').map(Number);
        const dt = calendarDateTimeToLuxon({ year: fromDate.year, month: fromDate.month, day: fromDate.day, hour, minute }, timezone);
        return toUTCISO(dt);
    }

    function getToDateTimeUTC(): string {
        const [hour, minute] = (toTime || '23:59').split(':').map(Number);
        const dt = calendarDateTimeToLuxon({ year: toDate.year, month: toDate.month, day: toDate.day, hour, minute }, timezone).endOf('minute');
        return toUTCISO(dt);
    }

    function handleTimeRangeChange(from: { date: CalendarDate; time: string }, to: { date: CalendarDate; time: string }, preset: string | null) {
        fromDate = from.date;
        fromTime = from.time;
        toDate = to.date;
        toTime = to.time;
        selectedPreset = preset;
        page = 1;
        loadData(true);
    }

    function shortId(id: string): string {
        return id.split('-')[0] ?? id.slice(0, 8);
    }

    const ABANDONED_AFTER_MS = 15 * 60_000;

    function durationLabel(s: Session): string {
        if (s.endedAt) return formatDuration(s.duration);
        const startedMs = Date.parse(s.startedAt);
        if (Number.isFinite(startedMs) && Date.now() - startedMs >= ABANDONED_AFTER_MS) {
            return 'Abandoned';
        }
        return 'in progress';
    }

    async function loadData(pushToHistory = true) {
        loading = true;
        error = '';

        if (selectedPreset) {
            const range = getTimeRangeFromPreset(selectedPreset, timezone);
            fromDate = dateToCalendarDate(range.from, timezone);
            toDate = dateToCalendarDate(range.to, timezone);
            fromTime = dateToTimeString(range.from, timezone);
            toTime = dateToTimeString(range.to, timezone);
        }

        updateTimeRangeUrl(pushToHistory);

        try {
            const requestBody = {
                fromDate: getFromDateTimeUTC(),
                toDate: getToDateTimeUTC(),
                orderBy,
                sortDirection,
                search: searchQuery.trim(),
                attributeFilters,
                pagination: { page, pageSize }
            };

            const response = await api.post('/sessions', requestBody, { projectId: projectsState.currentProjectId ?? undefined });

            sessions = response.data || [];
            total = response.pagination.total;
            totalPages = response.pagination.totalPages;
        } catch (e: any) {
            console.error(e);
            error = e.message || 'Failed to load data';
        } finally {
            loading = false;
        }
    }

    function handlePageChange(newPage: number) {
        if (newPage >= 1 && newPage <= totalPages) {
            page = newPage;
            loadData(false);
        }
    }

    function handlePageSizeChange(newPageSize: number) {
        pageSize = newPageSize;
        page = 1;
        loadData(false);
    }

    function handleSort(field: SortField) {
        const newSort = handleSortClick(field, orderBy, sortDirection);
        orderBy = newSort.field as SortField;
        sortDirection = newSort.direction;
        setSortState(SORT_STORAGE_KEY, newSort);
        page = 1;
        loadData(false);
    }

    onMount(() => {
        window.addEventListener('popstate', handlePopState);
        loadData(false);
    });

    onDestroy(() => {
        if (typeof window !== 'undefined') {
            window.removeEventListener('popstate', handlePopState);
        }
    });
</script>

<div class="space-y-4">
    <div class="flex flex-col gap-4 sm:flex-row sm:justify-between">
        <div class="flex items-start gap-2">
            <PageHeader title="Sessions" />
            <Tooltip.Root>
                <Tooltip.Trigger class="mt-2 text-muted-foreground/60 hover:text-muted-foreground">
                    <Info class="h-4 w-4" />
                </Tooltip.Trigger>
                <Tooltip.Content side="right" class="max-w-xs">
                    <p class="text-xs">
                        Sessions are full recordings of user interactions and are only recorded when manually enabled
                        (<code class="font-mono">recordAllSessions: true</code> in the SDK init options).
                    </p>
                </Tooltip.Content>
            </Tooltip.Root>
        </div>
        <div class="flex flex-col">
            <TimeRangePicker
                bind:fromDate
                bind:toDate
                bind:fromTime
                bind:toTime
                bind:preset={selectedPreset}
                onApply={handleTimeRangeChange}
            />
        </div>
    </div>

    <SearchBar
        placeholder="Search by session ID..."
        bind:value={searchQuery}
        onSearch={handleSearch}
    />

    <div class="flex flex-wrap items-center gap-2">
        <button
            type="button"
            class="inline-flex items-center gap-1 rounded-full border border-dashed px-3 py-0.5 text-xs font-medium text-muted-foreground transition-colors hover:border-foreground/40 hover:text-foreground"
            onclick={openAddFilterDialog}
            disabled={loading}
        >
            <Plus class="h-3 w-3" />
            Add filter
        </button>
        {#each attributeFilters as f, i (i)}
            <span class="inline-flex items-center gap-1 rounded-full bg-muted px-2 py-0.5 text-xs font-mono">
                <span class="text-muted-foreground">{f.key}</span>
                <span>=</span>
                <span>{f.value || '""'}</span>
                <button
                    type="button"
                    aria-label="Remove filter"
                    class="ml-1 text-muted-foreground hover:text-foreground"
                    onclick={() => removeAttributeFilter(i)}
                >
                    <X class="h-3 w-3" />
                </button>
            </span>
        {/each}
    </div>

    <AlertDialog.Root open={addFilterOpen} onOpenChange={(open) => (addFilterOpen = open)}>
        <AlertDialog.Content>
            <AlertDialog.Header>
                <AlertDialog.Title>Add attribute filter</AlertDialog.Title>
                <AlertDialog.Description>
                    Match sessions by an exact attribute value. Keys come from what the SDK auto-collects (<code class="font-mono">url</code>, <code class="font-mono">userAgent</code>, <code class="font-mono">viewport</code>, <code class="font-mono">client.ip</code>, …) plus anything you attach yourself.
                </AlertDialog.Description>
            </AlertDialog.Header>
            <div class="flex flex-col gap-3">
                <label class="flex flex-col gap-1 text-sm">
                    <span class="font-medium">Attribute key</span>
                    <Input
                        placeholder="userAgent"
                        bind:value={dialogKey}
                        onkeydown={handleDialogKeydown}
                    />
                </label>
                <label class="flex flex-col gap-1 text-sm">
                    <span class="font-medium">Value</span>
                    <Input
                        placeholder="Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)..."
                        bind:value={dialogValue}
                        onkeydown={handleDialogKeydown}
                    />
                </label>
                {#if dialogError}
                    <p class="text-xs text-red-500">{dialogError}</p>
                {/if}
            </div>
            <AlertDialog.Footer>
                <Button variant="outline" onclick={() => (addFilterOpen = false)}>Cancel</Button>
                <Button onclick={submitDialogFilter}>
                    <Plus class="h-4 w-4" /> Add filter
                </Button>
            </AlertDialog.Footer>
        </AlertDialog.Content>
    </AlertDialog.Root>

    <div class="rounded-md border overflow-hidden">
        <Table.Root>
            {#if loading}
            <Table.Body>
                <Table.Row>
                    <Table.Cell colspan={4} class="h-48">
                        <div class="flex justify-center items-center h-full">
                            <LoadingCircle size="xlg" />
                        </div>
                    </Table.Cell>
                </Table.Row>
            </Table.Body>
            {:else if error}
            <Table.Body>
                <Table.Row>
                    <Table.Cell colspan={4} class="h-24 text-center text-red-500">
                        {error}
                    </Table.Cell>
                </Table.Row>
            </Table.Body>
            {:else if sessions.length === 0}
            <Table.Body>
                <TableEmptyState colspan={4} message="No sessions recorded yet. Enable recordAllSessions in the SDK to start capturing them." />
            </Table.Body>
            {:else}
            <Table.Header>
                <Table.Row>
                    <TracewayTableHeader label="Session" tooltip="Session UUID. Open to play back the recording." />
                    <TracewayTableHeader
                        label="Started"
                        tooltip="When the session began"
                        sortField="started_at"
                        currentSortField={orderBy}
                        {sortDirection}
                        onSort={(field) => handleSort(field as SortField)}
                        class="w-[160px]"
                    />
                    <TracewayTableHeader
                        label="Duration"
                        tooltip="Wall-clock length of the session"
                        sortField="duration"
                        currentSortField={orderBy}
                        {sortDirection}
                        onSort={(field) => handleSort(field as SortField)}
                        class="w-[120px]"
                    />
                    <TracewayTableHeader label="Version" tooltip="App version reported by the SDK" class="w-[120px]" />
                </Table.Row>
            </Table.Header>
            <Table.Body>
                {#each sessions as session}
                    <Table.Row
                        class="cursor-pointer hover:bg-muted/50"
                        onclick={createRowClickHandler(`${resolve(`/sessions/${session.id}`)}?t=${encodeURIComponent(session.startedAt)}`, 'preset', 'from', 'to')}
                    >
                        <Table.Cell class="font-mono text-sm">{shortId(session.id)}</Table.Cell>
                        <Table.Cell class="text-sm">{formatRelativeTime(session.startedAt, timezone)}</Table.Cell>
                        <Table.Cell class="font-mono text-sm tabular-nums">
                            {durationLabel(session)}
                        </Table.Cell>
                        <Table.Cell class="font-mono text-sm">{session.appVersion || '—'}</Table.Cell>
                    </Table.Row>
                {/each}
            </Table.Body>
            {/if}
        </Table.Root>
    </div>

    <PaginationFooter
        currentPage={page}
        {totalPages}
        {pageSize}
        totalItems={total}
        onPageChange={handlePageChange}
        onPageSizeChange={handlePageSizeChange}
        {loading}
        itemLabel="session"
    />
</div>
