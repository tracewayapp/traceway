<script lang="ts">
	import { resolve } from '$app/paths';
    import { page } from '$app/state';
    import { ErrorDisplay } from '$lib/components/ui/error-display';
	import { createRowClickHandler } from '$lib/utils/navigation';

    const status = $derived(page.status);
    const errorMessage = $derived(page.error?.message);
</script>

<ErrorDisplay
    {status}
    title={status === 404 ? 'Page Not Found' : 'Something Went Wrong'}
    description={errorMessage || (status === 404
        ? "The page you're looking for doesn't exist or may have been moved."
        : 'An unexpected error occurred. Please try again.')}
    onBack={createRowClickHandler(resolve('/'))}
    backLabel="Go to Dashboard"
/>
