<script lang="ts">
    import { goto } from '$app/navigation';
    import { Button } from "$lib/components/ui/button";
    import { ArrowLeft } from "lucide-svelte";

    interface Props {
        title: string;
        subtitle: string;
        backHref: string;
    }

    let { title, subtitle, backHref }: Props = $props();
</script>

<div class="flex items-center gap-4">
    <Button variant="ghost" size="sm" onclick={() => goto(backHref)} class="h-8 w-8 p-0">
        <ArrowLeft class="h-4 w-4" />
    </Button>
    <div>
        <h2 class="font-mono text-2xl font-bold tracking-tight">
            {title}
        </h2>
        <p class="text-muted-foreground text-sm">{subtitle}</p>
    </div>
</div>
