<script lang="ts">
    import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "$lib/components/ui/card";
    import { Label } from "$lib/components/ui/label";
    import { Button } from "$lib/components/ui/button";
    import * as Select from "$lib/components/ui/select";
    import { organizationState } from '$lib/state/organization.svelte';
    import { authState } from '$lib/state/auth.svelte';
    import { toast } from 'svelte-sonner';

    const organization = $derived(organizationState.currentOrganization);
    const canManage = $derived(organizationState.canManage);

    let selectedTimezone = $state(organization?.timezone || 'UTC');
    let saving = $state(false);

    $effect(() => {
        if (organization?.timezone) {
            selectedTimezone = organization.timezone;
        }
    });

    const hasChanges = $derived(selectedTimezone !== organization?.timezone);

    const timezoneGroups = [
        {
            label: 'Common',
            timezones: [
                { value: 'UTC', label: 'UTC (Coordinated Universal Time)' },
                { value: 'America/New_York', label: 'US Eastern (New York)' },
                { value: 'America/Chicago', label: 'US Central (Chicago)' },
                { value: 'America/Denver', label: 'US Mountain (Denver)' },
                { value: 'America/Los_Angeles', label: 'US Pacific (Los Angeles)' },
            ]
        },
        {
            label: 'Europe',
            timezones: [
                { value: 'Europe/London', label: 'London (GMT/BST)' },
                { value: 'Europe/Paris', label: 'Paris (CET/CEST)' },
                { value: 'Europe/Berlin', label: 'Berlin (CET/CEST)' },
                { value: 'Europe/Belgrade', label: 'Belgrade (CET/CEST)' },
                { value: 'Europe/Amsterdam', label: 'Amsterdam (CET/CEST)' },
                { value: 'Europe/Stockholm', label: 'Stockholm (CET/CEST)' },
                { value: 'Europe/Zurich', label: 'Zurich (CET/CEST)' },
                { value: 'Europe/Madrid', label: 'Madrid (CET/CEST)' },
                { value: 'Europe/Rome', label: 'Rome (CET/CEST)' },
                { value: 'Europe/Athens', label: 'Athens (EET/EEST)' },
                { value: 'Europe/Moscow', label: 'Moscow (MSK)' },
            ]
        },
        {
            label: 'Asia & Pacific',
            timezones: [
                { value: 'Asia/Dubai', label: 'Dubai (GST)' },
                { value: 'Asia/Kolkata', label: 'India (IST)' },
                { value: 'Asia/Singapore', label: 'Singapore (SGT)' },
                { value: 'Asia/Hong_Kong', label: 'Hong Kong (HKT)' },
                { value: 'Asia/Shanghai', label: 'Shanghai (CST)' },
                { value: 'Asia/Tokyo', label: 'Tokyo (JST)' },
                { value: 'Asia/Seoul', label: 'Seoul (KST)' },
                { value: 'Australia/Sydney', label: 'Sydney (AEST/AEDT)' },
                { value: 'Australia/Melbourne', label: 'Melbourne (AEST/AEDT)' },
                { value: 'Pacific/Auckland', label: 'Auckland (NZST/NZDT)' },
            ]
        },
        {
            label: 'Americas',
            timezones: [
                { value: 'America/Toronto', label: 'Toronto (EST/EDT)' },
                { value: 'America/Vancouver', label: 'Vancouver (PST/PDT)' },
                { value: 'America/Mexico_City', label: 'Mexico City (CST/CDT)' },
                { value: 'America/Sao_Paulo', label: 'São Paulo (BRT)' },
                { value: 'America/Buenos_Aires', label: 'Buenos Aires (ART)' },
            ]
        }
    ];

    function getTimezoneLabel(value: string): string {
        for (const group of timezoneGroups) {
            const tz = group.timezones.find(t => t.value === value);
            if (tz) return tz.label;
        }
        return value;
    }

    async function handleSave() {
        if (!organization) return;

        saving = true;
        try {
            await organizationState.updateTimezone(organization.id, selectedTimezone);
            authState.updateOrganizationTimezone(organization.id, selectedTimezone);
            toast.success('Successfully updated the Timezone', { position: 'top-center' });
        } catch (e: unknown) {
            const errorMessage = e instanceof Error ? e.message : 'Failed to update timezone';
            toast.error(errorMessage);
        } finally {
            saving = false;
        }
    }
</script>

<Card>
    <CardHeader>
        <CardTitle>Organization Details</CardTitle>
        <CardDescription>Information about your organization</CardDescription>
    </CardHeader>
    <CardContent class="space-y-4">
        {#if organization}
            <div class="grid gap-4">
                <div class="grid grid-cols-4 items-center gap-4">
                    <Label class="text-right text-muted-foreground">Name</Label>
                    <div class="col-span-3">{organization.name}</div>
                </div>
                <div class="grid grid-cols-4 items-center gap-4">
                    <Label class="text-right text-muted-foreground">Timezone</Label>
                    <div class="col-span-3">
                        {#if canManage}
                            <div class="flex items-center gap-2">
                                <Select.Root type="single" bind:value={selectedTimezone}>
                                    <Select.Trigger class="w-[280px]">
                                        {getTimezoneLabel(selectedTimezone)}
                                    </Select.Trigger>
                                    <Select.Content class="max-h-[300px]">
                                        {#each timezoneGroups as group}
                                            <Select.Group>
                                                <Select.GroupHeading>{group.label}</Select.GroupHeading>
                                                {#each group.timezones as tz}
                                                    <Select.Item value={tz.value}>{tz.label}</Select.Item>
                                                {/each}
                                            </Select.Group>
                                        {/each}
                                    </Select.Content>
                                </Select.Root>
                                {#if hasChanges}
                                    <Button onclick={handleSave} disabled={saving} size="sm">
                                        {saving ? 'Saving...' : 'Save'}
                                    </Button>
                                {/if}
                            </div>
                            <p class="text-xs text-muted-foreground mt-1">
                                All team members will see timestamps in this timezone
                            </p>
                        {:else}
                            <span>{getTimezoneLabel(organization.timezone)}</span>
                        {/if}
                    </div>
                </div>
            </div>
        {:else}
            <p class="text-muted-foreground">Organization information not available</p>
        {/if}
    </CardContent>
</Card>
