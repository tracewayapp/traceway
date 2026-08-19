<script lang="ts">
	import * as Tabs from '$lib/components/ui/tabs';
	import { setSetupMode, type SetupMode } from '$lib/utils/setup-storage';
	import { underlineTabTriggerClass, underlineTabListClass } from '$lib/utils/tabs';

	let { mode, onModeChange }: { mode: SetupMode; onModeChange: (mode: SetupMode) => void } =
		$props();

	function handleChange(value: string) {
		if (value === 'ai' || value === 'manual') {
			setSetupMode(value);
			onModeChange(value);
		}
	}
</script>

<Tabs.Root value={mode} onValueChange={handleChange}>
	<Tabs.List class={underlineTabListClass}>
		<Tabs.Trigger value="ai" class={underlineTabTriggerClass}>AI</Tabs.Trigger>
		<Tabs.Trigger value="manual" class={underlineTabTriggerClass}>Manual</Tabs.Trigger>
	</Tabs.List>
</Tabs.Root>
