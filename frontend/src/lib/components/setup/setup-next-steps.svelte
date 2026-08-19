<script lang="ts">
	import bash from 'svelte-highlight/languages/bash';
	import { Rocket } from '@lucide/svelte';
	import {
		credentialValue,
		substituteToken,
		type SetupDraft,
		type SetupDraftProject,
		type SetupPlanProject
	} from '$lib/utils/setup-plan';
	import CopyableCodeBlock from './copyable-code-block.svelte';
	import CopyableInline from './copyable-inline.svelte';

	let { draft }: { draft: SetupDraft } = $props();

	type NextStep = {
		planned: SetupPlanProject;
		project: SetupDraftProject;
		instructions: string;
		value: string;
	};

	const steps = $derived.by((): NextStep[] => {
		const byName = new Map((draft.projects ?? []).map((p) => [p.name, p]));
		const result: NextStep[] = [];
		for (const planned of draft.payload.projects) {
			if (!planned.deployment) continue;
			const project = byName.get(planned.name);
			if (!project) continue;
			const value = credentialValue(planned, project);
			result.push({
				planned,
				project,
				instructions: substituteToken(planned.deployment.instructions, value),
				value
			});
		}
		return result;
	});
</script>

{#if steps.length > 0}
	<div class="rounded-md border bg-card">
		<div class="border-b px-4 py-3">
			<div class="flex items-center gap-3">
				<Rocket class="h-5 w-5 text-primary" />
				<h3 class="font-semibold">Next Steps: Deployment Credentials</h3>
			</div>
			<p class="mt-1 ml-8 text-sm text-muted-foreground">
				These credentials can't be written into your code by the agent because of how the app is
				deployed. Add them to your hosting platform.
			</p>
		</div>
		<div class="space-y-4 p-4">
			{#each steps as step (step.project.id)}
				<div class="space-y-2">
					<div class="text-sm font-medium">
						{step.planned.name}
						<span class="text-muted-foreground">on {step.planned.deployment?.platform}</span>
					</div>
					<CopyableCodeBlock code={step.instructions} language={bash} />
					{#if !step.planned.deployment?.instructions.includes('<token>')}
						<div class="space-y-1">
							<div class="text-xs text-muted-foreground">
								Credential value{step.planned.envVar ? ` for ${step.planned.envVar}` : ''}:
							</div>
							<CopyableInline value={step.value} />
						</div>
					{/if}
				</div>
			{/each}
		</div>
	</div>
{/if}
