<script lang="ts">
    import type { Framework } from '$lib/state/projects.svelte';

    interface Props {
        framework: Framework | string;
        class?: string;
    }

    let { framework, class: className = 'size-4 shrink-0' }: Props = $props();
</script>

{#if framework === 'gin'}
    <img src="/gin.png" alt="Gin" class={className} />
{:else if framework === 'fiber'}
    <svg class={className} viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg">
        <rect x="1" y="3" width="10" height="2.5" rx="0.5" fill="#00ACD7"/>
        <rect x="1" y="6.75" width="14" height="2.5" rx="0.5" fill="#00ACD7"/>
        <rect x="1" y="10.5" width="6" height="2.5" rx="0.5" fill="#00ACD7"/>
    </svg>
{:else if framework === 'chi'}
    <img src="/chi.png" alt="Chi" class={className} />
{:else if framework === 'fasthttp'}
    <img src="/fasthttp.png" alt="FastHTTP" class={className} />
{:else if framework === 'stdlib'}
    <img src="/stdlib.png" alt="Standard Library" class={className} />
{:else}
    <img src="/custom.png" alt="Custom" class={className} />
{/if}
