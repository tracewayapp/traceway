<script lang="ts">
	import { TriangleAlert } from '@lucide/svelte';
	import * as Tooltip from '$lib/components/ui/tooltip';

	type Props = {
		terms: string[];
	};

	let { terms }: Props = $props();
</script>

<Tooltip.Provider>
	<Tooltip.Root>
		<Tooltip.Trigger>
			<span
				class="inline-flex items-center gap-1 rounded-full bg-red-500/15 px-2 py-0.5 text-xs font-medium text-red-600 dark:text-red-400"
			>
				<TriangleAlert class="h-3 w-3" />
				Flagged
			</span>
		</Tooltip.Trigger>
		<Tooltip.Content>
			<p class="max-w-xs text-xs">Matched terms: {terms.join(', ')}</p>
		</Tooltip.Content>
	</Tooltip.Root>
</Tooltip.Provider>
