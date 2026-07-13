<script lang="ts">
	import { Alert, AlertDescription, AlertTitle } from '$lib/components/ui/alert';
	import { CircleAlert } from '@lucide/svelte';

	let {
		error,
		title = 'Error',
		class: className
	}: {
		error: string | string[] | null | undefined;
		title?: string;
		class?: string;
	} = $props();

	const messages = $derived(
		(Array.isArray(error) ? error : (error ?? '').split('\n'))
			.map((message) => message.trim())
			.filter(Boolean)
	);
</script>

{#if messages.length > 0}
	<Alert
		variant="destructive"
		class="rounded-sm border-destructive bg-destructive text-white has-[>svg]:grid-cols-[calc(var(--spacing)*5)_1fr] *:data-[slot=alert-description]:text-white/90 [&>svg]:size-5 {className ??
			''}"
	>
		<CircleAlert />
		<AlertTitle class="min-h-5 text-base font-semibold">{title}</AlertTitle>
		<AlertDescription>
			<ul class="list-disc pl-4">
				{#each messages as message (message)}
					<li>{message}</li>
				{/each}
			</ul>
		</AlertDescription>
	</Alert>
{/if}
