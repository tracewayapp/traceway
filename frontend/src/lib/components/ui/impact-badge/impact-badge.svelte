<script lang="ts">
	import { TriangleAlert, CheckCircle } from 'lucide-svelte';
	import * as Tooltip from '$lib/components/ui/tooltip';

	let {
		score,
		showIcon,
		reason
	}: {
		score: number; // 0-1 impact score
		showIcon?: boolean;
		reason?: string;
	} = $props();

	// Map score to impact level
	type ImpactLevel = 'critical' | 'high' | 'medium' | 'good';
	const level = $derived<ImpactLevel>(
		score >= 0.75 ? 'critical' : score >= 0.5 ? 'high' : score >= 0.25 ? 'medium' : 'good'
	);

	// Default showIcon to true for critical and high levels
	const shouldShowIcon = $derived(showIcon ?? (level === 'critical' || level === 'high'));

	const config = $derived(() => {
		switch (level) {
			case 'critical':
				return {
					bg: 'bg-red-500/15',
					text: 'text-red-600 dark:text-red-400',
					label: 'Critical',
					icon: 'alert'
				};
			case 'high':
				return {
					bg: 'bg-orange-500/15',
					text: 'text-orange-600 dark:text-orange-400',
					label: 'High',
					icon: 'alert'
				};
			case 'medium':
				return {
					bg: 'bg-yellow-500/15',
					text: 'text-yellow-600 dark:text-yellow-500',
					label: 'Medium',
					icon: 'alert'
				};
			case 'good':
				return {
					bg: 'bg-green-500/15',
					text: 'text-green-600 dark:text-green-400',
					label: 'Good',
					icon: 'check'
				};
		}
	});
</script>

{#snippet badge()}
	<span
		class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium {config()
			?.bg} {config()?.text}"
	>
		{#if shouldShowIcon}
			{#if config()?.icon === 'check'}
				<CheckCircle class="h-3 w-3" />
			{:else}
				<TriangleAlert class="h-3 w-3" />
			{/if}
		{/if}
		{config()?.label}
	</span>
{/snippet}

{#if config()}
	{#if reason}
		<Tooltip.Root>
			<Tooltip.Trigger class="cursor-default">
				{@render badge()}
			</Tooltip.Trigger>
			<Tooltip.Content side="left">
				{reason}
			</Tooltip.Content>
		</Tooltip.Root>
	{:else}
		{@render badge()}
	{/if}
{/if}
