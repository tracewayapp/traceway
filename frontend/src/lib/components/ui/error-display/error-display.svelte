<script lang="ts">
	import { Button } from '$lib/components/ui/button';

	interface Props {
		status?: 404 | 400 | 422 | number;
		title?: string;
		description?: string;
		onBack?: (e: MouseEvent) => void;
		backLabel?: string;
		onRetry?: () => void;
		identifier?: string;
	}

	let {
		status = 404,
		title,
		description,
		onBack,
		backLabel = 'Go Back',
		onRetry,
		identifier
	}: Props = $props();

	const defaults: Record<number, { title: string; description: string }> = {
		404: {
			title: 'Not Found',
			description: "The resource you're looking for doesn't exist or may have been removed."
		},
		400: {
			title: 'Bad Request',
			description: 'The request was invalid. Please check your input and try again.'
		},
		422: {
			title: 'Validation Error',
			description: 'The data provided could not be processed. Please verify your input.'
		}
	};

	const displayTitle = $derived(title || defaults[status]?.title || 'Error');
	const displayDescription = $derived(
		description || defaults[status]?.description || 'Something went wrong.'
	);

	function handleBack(e: MouseEvent) {
		if (onBack) {
			onBack(e);
		} else {
			history.back();
		}
	}
</script>

<div class="flex flex-col items-center justify-center px-4 py-16">
	<div class="relative mb-8">
		<!-- Glowing background effect -->
		{#if status === 404}
			<div
				class="absolute inset-0 scale-150 rounded-full bg-gradient-to-br from-red-500 via-orange-500 to-yellow-500 opacity-20 blur-3xl"
			></div>
		{:else if status === 400}
			<div
				class="absolute inset-0 scale-150 rounded-full bg-gradient-to-br from-amber-500 via-orange-500 to-red-500 opacity-20 blur-3xl"
			></div>
		{:else if status === 422}
			<div
				class="absolute inset-0 scale-150 rounded-full bg-gradient-to-br from-purple-500 via-pink-500 to-red-500 opacity-20 blur-3xl"
			></div>
		{:else}
			<div class="absolute inset-0 scale-150 rounded-full bg-red-500 opacity-20 blur-3xl"></div>
		{/if}

		<!-- Icon container -->
		<div
			class="relative flex h-24 w-24 items-center justify-center rounded-2xl border border-border/50 bg-gradient-to-br from-muted/80 to-muted shadow-lg"
		>
			{#if status === 404}
				<!-- X in circle icon -->
				<svg
					xmlns="http://www.w3.org/2000/svg"
					width="48"
					height="48"
					viewBox="0 0 24 24"
					fill="none"
					stroke="currentColor"
					stroke-width="1.5"
					stroke-linecap="round"
					stroke-linejoin="round"
					class="text-muted-foreground"
				>
					<circle cx="12" cy="12" r="10" />
					<path d="m15 9-6 6" />
					<path d="m9 9 6 6" />
				</svg>
			{:else if status === 400}
				<!-- Alert triangle icon -->
				<svg
					xmlns="http://www.w3.org/2000/svg"
					width="48"
					height="48"
					viewBox="0 0 24 24"
					fill="none"
					stroke="currentColor"
					stroke-width="1.5"
					stroke-linecap="round"
					stroke-linejoin="round"
					class="text-amber-500"
				>
					<path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3" />
					<path d="M12 9v4" />
					<path d="M12 17h.01" />
				</svg>
			{:else if status === 422}
				<!-- Alert circle icon -->
				<svg
					xmlns="http://www.w3.org/2000/svg"
					width="48"
					height="48"
					viewBox="0 0 24 24"
					fill="none"
					stroke="currentColor"
					stroke-width="1.5"
					stroke-linecap="round"
					stroke-linejoin="round"
					class="text-purple-500"
				>
					<circle cx="12" cy="12" r="10" />
					<path d="M12 8v4" />
					<path d="M12 16h.01" />
				</svg>
			{:else}
				<!-- Generic error icon -->
				<svg
					xmlns="http://www.w3.org/2000/svg"
					width="48"
					height="48"
					viewBox="0 0 24 24"
					fill="none"
					stroke="currentColor"
					stroke-width="1.5"
					stroke-linecap="round"
					stroke-linejoin="round"
					class="text-red-500"
				>
					<path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3" />
					<path d="M12 9v4" />
					<path d="M12 17h.01" />
				</svg>
			{/if}
		</div>
	</div>

	<div class="max-w-md space-y-3 text-center">
		<h2 class="text-2xl font-semibold tracking-tight">{displayTitle}</h2>
		<p class="leading-relaxed text-muted-foreground">{displayDescription}</p>
	</div>

	<div class="mt-8 flex items-center gap-3">
		<Button variant="outline" onclick={handleBack}>
			<svg
				xmlns="http://www.w3.org/2000/svg"
				width="16"
				height="16"
				viewBox="0 0 24 24"
				fill="none"
				stroke="currentColor"
				stroke-width="2"
				stroke-linecap="round"
				stroke-linejoin="round"
				class="mr-2"
			>
				<path d="m12 19-7-7 7-7" /><path d="M19 12H5" />
			</svg>
			{backLabel}
		</Button>
		{#if onRetry}
			<Button onclick={onRetry}>
				<svg
					xmlns="http://www.w3.org/2000/svg"
					width="16"
					height="16"
					viewBox="0 0 24 24"
					fill="none"
					stroke="currentColor"
					stroke-width="2"
					stroke-linecap="round"
					stroke-linejoin="round"
					class="mr-2"
				>
					<path d="M21 12a9 9 0 0 0-9-9 9.75 9.75 0 0 0-6.74 2.74L3 8" />
					<path d="M3 3v5h5" />
					<path d="M3 12a9 9 0 0 0 9 9 9.75 9.75 0 0 0 6.74-2.74L21 16" />
					<path d="M16 16h5v5" />
				</svg>
				Try Again
			</Button>
		{/if}
	</div>

	{#if identifier}
		<div class="mt-8 rounded-md border border-border/50 bg-muted/50 px-4 py-2">
			<code class="font-mono text-xs text-muted-foreground">
				{identifier}
			</code>
		</div>
	{/if}
</div>
