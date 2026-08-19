<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import * as Tooltip from '$lib/components/ui/tooltip';

	type Props = {
		mixed?: boolean;
	};

	let { mixed = false }: Props = $props();

	const label = $derived(mixed ? 'Mixed' : 'Non Root');
	const tooltip = $derived(
		mixed
			? 'Some runs were triggered by another trace. See the distributed trace view.'
			: 'Triggered by another trace. See the distributed trace view.'
	);
</script>

<Tooltip.Provider>
	<Tooltip.Root>
		<Tooltip.Trigger>
			<Badge
				variant="outline"
				class="ml-2 border-amber-500 bg-amber-50 text-xs text-amber-700 dark:border-yellow-500 dark:bg-transparent dark:text-yellow-400"
			>
				{label}
			</Badge>
		</Tooltip.Trigger>
		<Tooltip.Content>
			{tooltip}
		</Tooltip.Content>
	</Tooltip.Root>
</Tooltip.Provider>
