<script lang="ts">
	import * as Select from '$lib/components/ui/select';

	interface Props {
		organizations: { id: number; name: string }[];
		currentOrganizationId: number | null;
		currentOrganizationName?: string;
		onChange: (id: number) => void;
	}

	let { organizations, currentOrganizationId, currentOrganizationName, onChange }: Props = $props();
</script>

<div class="flex w-full items-center gap-2 sm:w-auto">
	<span class="text-sm text-muted-foreground">Organization</span>
	<Select.Root
		type="single"
		value={currentOrganizationId !== null ? String(currentOrganizationId) : undefined}
		onValueChange={(val) => {
			if (val) onChange(Number(val));
		}}
	>
		<Select.Trigger class="min-w-0 flex-1 sm:w-[220px] sm:flex-none">
			{currentOrganizationName || 'Select organization'}
		</Select.Trigger>
		<Select.Content>
			{#each organizations as org (org.id)}
				<Select.Item value={String(org.id)}>{org.name}</Select.Item>
			{/each}
		</Select.Content>
	</Select.Root>
</div>
