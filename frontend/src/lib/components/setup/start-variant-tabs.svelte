<script lang="ts">
	import * as Tabs from '$lib/components/ui/tabs';
	import type { StartVariant } from '$lib/utils/sdk-start';
	import { underlineTabTriggerClass, underlineTabListClass } from '$lib/utils/tabs';

	let {
		variants,
		value = $bindable()
	}: { variants: { id: StartVariant; label: string }[]; value: StartVariant } = $props();
</script>

{#if variants.length > 1}
	<Tabs.Root {value} onValueChange={(v) => (value = v as StartVariant)}>
		<Tabs.List class={underlineTabListClass}>
			{#each variants as variant (variant.id)}
				<Tabs.Trigger value={variant.id} class={underlineTabTriggerClass}>
					{variant.label}
				</Tabs.Trigger>
			{/each}
		</Tabs.List>
	</Tabs.Root>
{/if}
