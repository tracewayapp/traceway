<script lang="ts">
	import type { Snippet } from 'svelte';
	import { Button } from '$lib/components/ui/button';
	import { ArrowLeft } from '@lucide/svelte';

	interface Props {
		title: string;
		subtitle?: string;
		description?: string;
		onBack?: (e: MouseEvent) => void;
		trailing?: Snippet;
		meta?: Snippet;
		actions?: Snippet;
	}

	let { title, subtitle, description, onBack, trailing, meta, actions }: Props = $props();
</script>

<!-- Container query rather than viewport breakpoints so the actions row accounts
     for the sidebar (and works inside sheets): below @2xl the actions become their
     own full-width stacked rows. -->
<div class="@container">
	<div class="flex flex-wrap items-start justify-between gap-x-4 gap-y-3">
		<div class="flex min-w-0 items-start gap-4">
			{#if onBack}
				<Button variant="ghost" size="sm" onclick={onBack} class="mt-1 h-8 w-8 shrink-0 p-0">
					<ArrowLeft class="h-4 w-4" />
					<span class="sr-only">Back</span>
				</Button>
			{/if}
			<div class="min-w-0">
				<div class="flex flex-wrap items-center gap-2">
					<h1
						class="min-w-0 text-3xl font-semibold tracking-tight text-balance [overflow-wrap:anywhere]"
					>
						{title}
					</h1>
					{#if trailing}{@render trailing()}{/if}
				</div>
				{#if subtitle}
					<p class="mt-1 font-mono text-sm break-all text-muted-foreground">{subtitle}</p>
				{/if}
				{#if description}
					<p class="mt-1 max-w-prose text-sm text-muted-foreground">{description}</p>
				{/if}
			</div>
		</div>
		{#if actions}
			<div class="flex w-full flex-col gap-2 @2xl:w-auto @2xl:flex-row @2xl:items-center">
				{@render actions()}
			</div>
		{/if}
	</div>
	{#if meta}
		<div class="mt-3 flex flex-wrap items-center gap-3">
			{@render meta()}
		</div>
	{/if}
</div>
