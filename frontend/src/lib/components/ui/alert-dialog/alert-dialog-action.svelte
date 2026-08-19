<script lang="ts">
	import { AlertDialog as AlertDialogPrimitive } from 'bits-ui';
	import { buttonVariants, type ButtonVariant } from '$lib/components/ui/button/index.js';
	import { cn } from '$lib/utils.js';

	let {
		ref = $bindable(null),
		class: className,
		variant = 'default',
		...restProps
	}: AlertDialogPrimitive.ActionProps & { variant?: ButtonVariant } = $props();
</script>

<AlertDialogPrimitive.Action
	bind:ref
	class={cn(buttonVariants({ variant }), className)}
	{...restProps}
/>
