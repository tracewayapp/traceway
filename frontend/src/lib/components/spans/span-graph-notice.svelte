<script lang="ts">
	import type { SpanGraphStatus } from '$lib/types/spans';
	import { WarningCallout } from '$lib/components/ui/warning-callout';

	let { status }: { status?: SpanGraphStatus | null } = $props();
</script>

{#if status && status.state !== 'complete'}
	<WarningCallout
		title={status.state === 'unavailable'
			? 'Spans are temporarily unavailable'
			: status.reasons?.includes('row_limit')
				? 'This trace is too large to show in full'
				: 'Some trace details could not be loaded'}
	>
		{#if status.state === 'unavailable'}
			The trace could not be loaded within the read limits. Refresh to try again.
		{:else if status.reasons?.includes('most_important')}
			The available spans prioritize errors, entry points and slowest spans. Other spans may be
			missing.
		{:else if status.reasons?.includes('row_limit')}
			The spans shown are only part of the trace.
		{:else}
			The available spans are shown, but some attributes could not be loaded.
		{/if}
		{#if status.state === 'unavailable' || status.reasons?.includes('row_limit')}
			Exceptions attached to missing spans may also be absent from this view.
		{/if}
		{#if status.omittedAttributes}
			Attributes are missing from {status.omittedAttributes}
			{status.omittedAttributes === 1 ? 'span' : 'spans'}.
		{/if}
	</WarningCallout>
{/if}
