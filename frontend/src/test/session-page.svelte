<script lang="ts">
	import * as Tooltip from '$lib/components/ui/tooltip';
	import SessionPage from '../routes/sessions/[sessionId]/+page.svelte';
</script>

<Tooltip.Provider>
	<SessionPage />
</Tooltip.Provider>
