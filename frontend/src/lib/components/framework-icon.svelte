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
{:else if framework === 'react'}
    <svg class={className} viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg">
        <circle cx="8" cy="8" r="1.5" fill="#61DAFB"/>
        <ellipse cx="8" cy="8" rx="6.5" ry="2.5" stroke="#61DAFB" stroke-width="0.8" fill="none"/>
        <ellipse cx="8" cy="8" rx="6.5" ry="2.5" stroke="#61DAFB" stroke-width="0.8" fill="none" transform="rotate(60 8 8)"/>
        <ellipse cx="8" cy="8" rx="6.5" ry="2.5" stroke="#61DAFB" stroke-width="0.8" fill="none" transform="rotate(120 8 8)"/>
    </svg>
{:else if framework === 'svelte'}
    <img src="/svelte.png" alt="Svelte" class={className} />
{:else if framework === 'vuejs'}
    <svg class={className} viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg">
        <path d="M12.8 1H10.4L8 5L5.6 1H3.2L8 9.5L12.8 1Z" fill="#41B883"/>
        <path d="M15 1H12.8L8 9.5L3.2 1H1L8 13L15 1Z" fill="#41B883" opacity="0.6"/>
        <path d="M12.8 1H10.4L8 5L5.6 1H3.2L8 9.5L12.8 1Z" fill="#35495E"/>
    </svg>
{:else if framework === 'nextjs'}
    <img src="/nextjs.png" alt="Next.js" class={className} />
{:else if framework === 'nestjs'}
    <img src="/nestjs.png" alt="NestJS" class={className} />
{:else if framework === 'express'}
    <img src="/express.png" alt="Express" class={className} />
{:else if framework === 'remix'}
    <img src="/remix.png" alt="Remix" class={className} />
{:else}
    <img src="/custom.png" alt="Custom" class={className} />
{/if}
