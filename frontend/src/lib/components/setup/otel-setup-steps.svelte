<script lang="ts">
	import * as Tabs from '$lib/components/ui/tabs';
	import bash from 'svelte-highlight/languages/bash';
	import go from 'svelte-highlight/languages/go';
	import javascript from 'svelte-highlight/languages/javascript';
	import typescript from 'svelte-highlight/languages/typescript';
	import python from 'svelte-highlight/languages/python';
	import gradle from 'svelte-highlight/languages/gradle';
	import csharp from 'svelte-highlight/languages/csharp';
	import ruby from 'svelte-highlight/languages/ruby';
	import yaml from 'svelte-highlight/languages/yaml';
	import {
		OTEL_TARGETS,
		getOtelSteps,
		type OtelStepLanguage,
		type OtelTargetId
	} from '$lib/utils/otel-setup';
	import {
		getOtelTarget,
		setOtelTarget,
		getOtelFramework,
		setOtelFramework
	} from '$lib/utils/setup-storage';
	import CopyableCodeBlock from './copyable-code-block.svelte';
	import SourceMapUploadCard from './source-map-upload-card.svelte';

	let { backendUrl, token }: { backendUrl: string; token: string } = $props();

	let target = $state(getOtelTarget());
	let framework = $state(getOtelFramework());

	const highlightLanguages = {
		bash,
		go,
		javascript,
		typescript,
		python,
		gradle,
		csharp,
		ruby,
		yaml
	};

	const targetDef = $derived(OTEL_TARGETS.find((t) => t.id === target) ?? OTEL_TARGETS[0]);
	const activeFramework = $derived(
		targetDef.frameworks.find((f) => f.id === framework)?.id ?? targetDef.frameworks[0]?.id ?? ''
	);
	const steps = $derived(getOtelSteps(targetDef.id, activeFramework, backendUrl, token));

	function handleTargetChange(value: string) {
		const next = OTEL_TARGETS.find((t) => t.id === value);
		if (next) {
			target = next.id;
			setOtelTarget(next.id);
		}
	}

	function handleFrameworkChange(value: string) {
		if (targetDef.frameworks.some((f) => f.id === value)) {
			framework = value;
			setOtelFramework(value);
		}
	}

	function languageFor(id: OtelStepLanguage | undefined) {
		return highlightLanguages[id ?? 'bash'];
	}
</script>

<div class="space-y-2">
	<p class="text-sm font-medium">Language</p>
	<Tabs.Root value={target} onValueChange={handleTargetChange}>
		<Tabs.List class="h-auto flex-wrap justify-start">
			{#each OTEL_TARGETS as t (t.id)}
				<Tabs.Trigger value={t.id}>{t.label}</Tabs.Trigger>
			{/each}
		</Tabs.List>
	</Tabs.Root>
	{#if targetDef.frameworks.length > 1}
		<p class="pt-1 text-sm font-medium">Framework</p>
		<Tabs.Root value={activeFramework} onValueChange={handleFrameworkChange}>
			<Tabs.List class="h-auto flex-wrap justify-start">
				{#each targetDef.frameworks as f (f.id)}
					<Tabs.Trigger value={f.id}>{f.label}</Tabs.Trigger>
				{/each}
			</Tabs.List>
		</Tabs.Root>
	{/if}
</div>

{#each steps as step, i (targetDef.id + activeFramework + step.title)}
	<div class="rounded-md border bg-card">
		<div class="border-b px-4 py-3">
			<div class="flex items-center gap-3">
				<div
					class="flex h-6 w-6 items-center justify-center rounded-full bg-primary text-sm font-medium text-primary-foreground"
				>
					{i + 1}
				</div>
				<h3 class="font-semibold">{step.title}</h3>
			</div>
			{#if step.description}
				<p class="mt-1 ml-9 text-sm text-muted-foreground">{step.description}</p>
			{/if}
		</div>
		{#if step.code}
			<div class="p-4">
				<CopyableCodeBlock code={step.code} language={languageFor(step.codeLanguage)} />
				{#if step.link}
					<p class="pt-2 text-xs text-muted-foreground">
						<a
							href={step.link.href}
							target="_blank"
							rel="noopener noreferrer"
							class="underline hover:text-foreground">{step.link.label}</a
						>
					</p>
				{/if}
			</div>
		{/if}
	</div>
{/each}

{#if target === 'nodejs'}
	<SourceMapUploadCard />
{/if}
