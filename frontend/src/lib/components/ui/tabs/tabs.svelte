<script lang="ts">
	import { Tabs as TabsPrimitive } from "bits-ui";

	let {
		value = $bindable(),
		...restProps
	}: TabsPrimitive.RootProps = $props();
</script>

<TabsPrimitive.Root bind:value {...restProps} />
