<script lang="ts">
	import * as Select from '$lib/components/ui/select';
	import { Label } from '$lib/components/ui/label';
	import { authState } from '$lib/state/auth.svelte';
	import { projectsState } from '$lib/state/projects.svelte';
	import SetupProjectsStep from '$lib/components/setup/setup-projects-step.svelte';

	const writableOrgs = $derived(authState.organizations.filter((o) => o.role !== 'readonly'));

	let selectedOrgId = $state<number | null>(null);

	$effect(() => {
		if (selectedOrgId !== null && writableOrgs.some((o) => o.id === selectedOrgId)) return;
		const currentOrgId = projectsState.currentProject?.organizationId;
		selectedOrgId = writableOrgs.some((o) => o.id === currentOrgId)
			? currentOrgId!
			: (writableOrgs[0]?.id ?? null);
	});

	const selectedOrgName = $derived(writableOrgs.find((o) => o.id === selectedOrgId)?.name ?? '');
</script>

<div class="mx-auto w-full max-w-2xl space-y-6">
	<div>
		<h1 class="text-2xl font-bold">Set Up Projects</h1>
		<p class="mt-1 text-sm text-muted-foreground">
			Let your coding agent propose the project setup for your approval, or create projects
			manually.
		</p>
	</div>

	{#if writableOrgs.length === 0}
		<p class="text-sm text-muted-foreground">
			You need an owner, admin, or user role in an organization to create projects.
		</p>
	{:else}
		{#if writableOrgs.length > 1}
			<div class="flex max-w-xs flex-col space-y-1.5">
				<Label>Organization</Label>
				<Select.Root
					type="single"
					value={selectedOrgId !== null ? String(selectedOrgId) : undefined}
					onValueChange={(val) => {
						if (val) selectedOrgId = Number(val);
					}}
				>
					<Select.Trigger class="w-full">
						{selectedOrgName || 'Select organization'}
					</Select.Trigger>
					<Select.Content>
						{#each writableOrgs as org (org.id)}
							<Select.Item value={String(org.id)}>{org.name}</Select.Item>
						{/each}
					</Select.Content>
				</Select.Root>
			</div>
		{/if}

		{#if selectedOrgId !== null}
			{#key selectedOrgId}
				<SetupProjectsStep organizationId={selectedOrgId} />
			{/key}
		{/if}
	{/if}
</div>
