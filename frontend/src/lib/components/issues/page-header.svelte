<script lang="ts">
    import { goto } from '$app/navigation';
    import { Button } from "$lib/components/ui/button";
    import { ArrowLeft } from "lucide-svelte";

    interface Props {
        title: string;
        subtitle?: string;
        onBack?: (e: MouseEvent) => void;
    }

    let { title, subtitle, onBack }: Props = $props();
</script>

<div class="flex items-start gap-4">
    {#if onBack}
        <Button variant="ghost" size="sm" onclick={onBack} class="h-8 w-8 p-0">
            <ArrowLeft class="h-4 w-4" />
        </Button>
    {/if}
    <div>
        <h2 class="font-mono text-2xl font-bold tracking-tight break-all">
            {title}
        </h2>
        {#if subtitle}
            <p class="text-muted-foreground text-sm">{subtitle}</p>
        {/if}
    </div>
</div>
