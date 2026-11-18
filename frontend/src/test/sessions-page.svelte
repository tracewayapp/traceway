<script lang="ts">
	import * as Tooltip from '$lib/components/ui/tooltip';
	import SessionsPage from '../routes/sessions/+page.svelte';
</script>

<Tooltip.Provider>
	<SessionsPage />
</Tooltip.Provider>
