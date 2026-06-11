<script lang="ts">
	import * as Tabs from '$lib/components/ui/tabs';
	import { setSetupMode, type SetupMode } from '$lib/utils/setup-storage';

	let { mode, onModeChange }: { mode: SetupMode; onModeChange: (mode: SetupMode) => void } =
		$props();

	const tabTriggerClass =
		'-mb-px rounded-none border-b-2 border-transparent bg-transparent px-0 pb-2.5 pt-0 text-sm font-medium text-muted-foreground shadow-none data-[state=active]:border-primary data-[state=active]:bg-transparent data-[state=active]:text-foreground data-[state=active]:shadow-none';

	function handleChange(value: string) {
		if (value === 'ai' || value === 'manual') {
			setSetupMode(value);
			onModeChange(value);
		}
	}
</script>

<Tabs.Root value={mode} onValueChange={handleChange}>
	<Tabs.List class="h-auto w-full justify-start gap-4 rounded-none border-b bg-transparent p-0">
		<Tabs.Trigger value="ai" class={tabTriggerClass}>AI</Tabs.Trigger>
		<Tabs.Trigger value="manual" class={tabTriggerClass}>Manual</Tabs.Trigger>
	</Tabs.List>
</Tabs.Root>
