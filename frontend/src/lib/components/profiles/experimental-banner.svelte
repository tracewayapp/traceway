<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { FlaskConical } from '@lucide/svelte';

	const ISSUES_URL = 'https://github.com/tracewayapp/traceway/issues';
</script>

<div
	class="flex items-center gap-3 rounded-md border border-blue-200 bg-blue-50 px-4 py-2.5 text-sm text-blue-900 dark:border-blue-900/50 dark:bg-blue-950/40 dark:text-blue-200"
>
	<FlaskConical class="size-4 shrink-0 text-blue-600 dark:text-blue-400" />
	<span class="min-w-0 flex-1">
		Profiles is experimental. We're actively building out continuous profiling and would love your
		feedback.
	</span>
	<Button
		href={ISSUES_URL}
		target="_blank"
		rel="noopener noreferrer"
		variant="ghost"
		size="sm"
		class="shrink-0 bg-blue-100 text-blue-800 hover:bg-blue-200 hover:text-blue-900 dark:bg-blue-900/50 dark:text-blue-200 dark:hover:bg-blue-900/70"
	>
		Give Feedback
	</Button>
</div>
