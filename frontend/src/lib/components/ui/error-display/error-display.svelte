<script lang="ts">
    import { goto } from '$app/navigation';
    import { Button } from "$lib/components/ui/button";

    interface Props {
        status?: 404 | 400 | 422 | number;
        title?: string;
        description?: string;
        backHref?: string;
        backLabel?: string;
        onRetry?: () => void;
        identifier?: string;
    }

    let {
        status = 404,
        title,
        description,
        backHref,
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
    const displayDescription = $derived(description || defaults[status]?.description || 'Something went wrong.');

    function handleBack() {
        if (backHref) {
            goto(backHref);
        } else {
            history.back();
        }
    }
</script>

<div class="flex flex-col items-center justify-center py-16 px-4">
    <div class="relative mb-8">
        <!-- Glowing background effect -->
        {#if status === 404}
            <div class="absolute inset-0 blur-3xl opacity-20 bg-gradient-to-br from-red-500 via-orange-500 to-yellow-500 rounded-full scale-150"></div>
        {:else if status === 400}
            <div class="absolute inset-0 blur-3xl opacity-20 bg-gradient-to-br from-amber-500 via-orange-500 to-red-500 rounded-full scale-150"></div>
        {:else if status === 422}
            <div class="absolute inset-0 blur-3xl opacity-20 bg-gradient-to-br from-purple-500 via-pink-500 to-red-500 rounded-full scale-150"></div>
        {:else}
            <div class="absolute inset-0 blur-3xl opacity-20 bg-red-500 rounded-full scale-150"></div>
        {/if}

        <!-- Icon container -->
        <div class="relative flex items-center justify-center w-24 h-24 rounded-2xl bg-gradient-to-br from-muted/80 to-muted border border-border/50 shadow-lg">
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
                    <circle cx="12" cy="12" r="10"/>
                    <path d="m15 9-6 6"/>
                    <path d="m9 9 6 6"/>
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
                    <path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3"/>
                    <path d="M12 9v4"/>
                    <path d="M12 17h.01"/>
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
                    <circle cx="12" cy="12" r="10"/>
                    <path d="M12 8v4"/>
                    <path d="M12 16h.01"/>
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
                    <path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3"/>
                    <path d="M12 9v4"/>
                    <path d="M12 17h.01"/>
                </svg>
            {/if}
        </div>
    </div>

    <div class="text-center space-y-3 max-w-md">
        <h2 class="text-2xl font-semibold tracking-tight">{displayTitle}</h2>
        <p class="text-muted-foreground leading-relaxed">{displayDescription}</p>
    </div>

    <div class="flex items-center gap-3 mt-8">
        <Button variant="outline" onclick={handleBack}>
            <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="mr-2">
                <path d="m12 19-7-7 7-7"/><path d="M19 12H5"/>
            </svg>
            {backLabel}
        </Button>
        {#if onRetry}
            <Button onclick={onRetry}>
                <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="mr-2">
                    <path d="M21 12a9 9 0 0 0-9-9 9.75 9.75 0 0 0-6.74 2.74L3 8"/>
                    <path d="M3 3v5h5"/>
                    <path d="M3 12a9 9 0 0 0 9 9 9.75 9.75 0 0 0 6.74-2.74L21 16"/>
                    <path d="M16 16h5v5"/>
                </svg>
                Try Again
            </Button>
        {/if}
    </div>

    {#if identifier}
        <div class="mt-8 px-4 py-2 rounded-lg bg-muted/50 border border-border/50">
            <code class="text-xs text-muted-foreground font-mono">
                {identifier}
            </code>
        </div>
    {/if}
</div>
