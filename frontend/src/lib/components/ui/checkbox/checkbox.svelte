<script lang="ts">
	import { Checkbox as CheckboxPrimitive } from "bits-ui";
	import CheckIcon from "@lucide/svelte/icons/check";
	import MinusIcon from "@lucide/svelte/icons/minus";
	import { cn } from "$lib/utils.js";

	type CheckedState = boolean | "indeterminate";

	let {
		ref = $bindable(null),
		checked = $bindable<CheckedState>(false),
		onCheckedChange,
		class: className,
		disabled = false,
		...restProps
	}: {
		ref?: HTMLButtonElement | null;
		checked?: CheckedState;
		onCheckedChange?: (checked: CheckedState) => void;
		class?: string;
		disabled?: boolean;
		"aria-label"?: string;
	} = $props();

	// Split checked state into checked and indeterminate for bits-ui
	const isChecked = $derived(checked === true);
	const isIndeterminate = $derived(checked === "indeterminate");

	function handleCheckedChange(value: boolean) {
		const newState: CheckedState = value;
		checked = newState;
		onCheckedChange?.(newState);
	}

	function handleIndeterminateChange(value: boolean) {
		if (value) {
			checked = "indeterminate";
			onCheckedChange?.("indeterminate");
		}
	}
</script>

<CheckboxPrimitive.Root
	bind:ref
	checked={isChecked}
	indeterminate={isIndeterminate}
	onCheckedChange={handleCheckedChange}
	onIndeterminateChange={handleIndeterminateChange}
	{disabled}
	class={cn(
		"peer size-4 shrink-0 rounded-[4px] border border-input bg-background shadow-sm transition-colors",
		"focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring",
		"disabled:cursor-not-allowed disabled:opacity-50",
		"data-[state=checked]:bg-primary data-[state=checked]:border-primary data-[state=checked]:text-primary-foreground",
		"data-[state=indeterminate]:bg-primary data-[state=indeterminate]:border-primary data-[state=indeterminate]:text-primary-foreground",
		className
	)}
	{...restProps}
>
	{#snippet children({ checked: isOn, indeterminate })}
		<span class="flex items-center justify-center text-current">
			{#if indeterminate}
				<MinusIcon class="size-3" />
			{:else if isOn}
				<CheckIcon class="size-3" />
			{/if}
		</span>
	{/snippet}
</CheckboxPrimitive.Root>
