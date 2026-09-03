<script lang="ts">
	import { untrack } from 'svelte';
	import { Plus, X } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { ErrorAlert } from '$lib/components/ui/error-alert';
	import { projectsState, type Framework, type ProjectWithToken } from '$lib/state/projects.svelte';
	import FrameworkCombobox from '$lib/components/framework-combobox.svelte';

	let {
		organizationId,
		initialFramework = 'opentelemetry',
		onCreated
	}: {
		organizationId: number;
		initialFramework?: Framework;
		onCreated: (projects: ProjectWithToken[]) => void;
	} = $props();

	type Row = { key: number; name: string; framework: Framework };

	let nextKey = 1;
	let rows = $state<Row[]>([{ key: 0, name: '', framework: untrack(() => initialFramework) }]);
	let loading = $state(false);
	let error = $state('');

	function addRow() {
		rows = [...rows, { key: nextKey++, name: '', framework: 'opentelemetry' }];
	}

	function removeRow(key: number) {
		rows = rows.filter((r) => r.key !== key);
	}

	async function handleSubmit(e: Event) {
		e.preventDefault();
		loading = true;
		error = '';
		try {
			const created = await projectsState.createProjects(
				organizationId,
				rows.map((r) => ({ name: r.name.trim(), framework: r.framework }))
			);
			toast.success('Successfully created the Projects', { position: 'top-center' });
			if (created.length > 0) {
				projectsState.selectProject(created[0].id);
			}
			rows = [{ key: nextKey++, name: '', framework: 'opentelemetry' }];
			onCreated(created);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to create projects';
		} finally {
			loading = false;
		}
	}
</script>

<form onsubmit={handleSubmit} class="rounded-md border bg-card">
	<div class="border-b px-4 py-3">
		<h3 class="font-semibold">Create Projects</h3>
		<p class="mt-1 text-sm text-muted-foreground">
			One project per deployed application: a backend uses OpenTelemetry; each browser or mobile app
			gets its own project.
		</p>
	</div>
	<div class="space-y-4 p-4">
		{#if error}
			<ErrorAlert {error} />
		{/if}

		<div class="hidden gap-3 sm:grid sm:grid-cols-[1fr_1fr_2rem]">
			<Label>Project Name</Label>
			<Label>Framework</Label>
			<span></span>
		</div>
		{#each rows as row (row.key)}
			<div class="grid grid-cols-1 gap-3 sm:grid-cols-[1fr_1fr_2rem] sm:items-center">
				<Input type="text" placeholder="My Application" bind:value={row.name} disabled={loading} />
				<FrameworkCombobox bind:value={row.framework} disabled={loading} />
				<div class="flex justify-end">
					{#if rows.length > 1}
						<Button
							type="button"
							variant="ghost"
							size="icon"
							onclick={() => removeRow(row.key)}
							disabled={loading}
							title="Remove project"
						>
							<X class="h-4 w-4" />
						</Button>
					{/if}
				</div>
			</div>
		{/each}

		<div class="flex items-center justify-between pt-1">
			<Button type="button" variant="ghost" size="sm" onclick={addRow} disabled={loading}>
				<Plus class="mr-2 h-4 w-4" />
				Add another project
			</Button>
			<Button type="submit" variant="success" disabled={loading}>
				{#if loading}
					Creating...
				{:else}
					<Plus class="mr-2 h-4 w-4" />
					New Projects
				{/if}
			</Button>
		</div>
	</div>
</form>
