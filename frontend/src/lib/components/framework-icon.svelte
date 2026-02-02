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
    <svg class={className} viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg">
        <path d="M13.2 2.8C11.8 0.8 9.1 0.2 7.1 1.4L3.8 3.6C2.9 4.2 2.3 5.1 2.1 6.1C2 6.9 2.1 7.7 2.5 8.4C2.2 8.9 2.1 9.5 2.1 10.1C2 11.1 2.4 12.1 3.1 12.8C4.5 14.8 7.2 15.4 9.2 14.2L12.5 12C13.4 11.4 14 10.5 14.2 9.5C14.3 8.7 14.2 7.9 13.8 7.2C14.1 6.7 14.2 6.1 14.2 5.5C14.3 4.5 13.9 3.5 13.2 2.8Z" fill="#FF3E00"/>
        <path d="M7.4 12.8C6.2 13.2 4.9 12.7 4.2 11.7C3.9 11.3 3.7 10.7 3.8 10.2L3.9 9.8L4.1 10.1C4.5 10.7 5.1 11.1 5.7 11.3L5.9 11.3C6.5 11.3 7.1 11 7.4 10.5C7.5 10.3 7.6 10 7.5 9.7C7.5 9.5 7.4 9.2 7.2 9.1L5.8 8.1C5 7.5 4.6 6.4 4.8 5.4C4.9 4.9 5.2 4.4 5.6 4.1L6.9 3.2C7.5 2.8 8.2 2.7 8.9 2.8C9.6 3 10.2 3.4 10.5 4C10.8 4.4 11 4.9 10.9 5.5L10.8 5.9L10.6 5.6C10.2 5 9.6 4.6 9 4.4L8.8 4.4C8.2 4.4 7.6 4.7 7.3 5.2C7.2 5.4 7.1 5.7 7.2 5.9C7.2 6.2 7.3 6.4 7.5 6.6L8.9 7.6C9.7 8.2 10.1 9.2 9.9 10.3C9.8 10.8 9.5 11.3 9.1 11.6L7.4 12.8Z" fill="white"/>
    </svg>
{:else if framework === 'vuejs'}
    <svg class={className} viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg">
        <path d="M12.8 1H10.4L8 5L5.6 1H3.2L8 9.5L12.8 1Z" fill="#41B883"/>
        <path d="M15 1H12.8L8 9.5L3.2 1H1L8 13L15 1Z" fill="#41B883" opacity="0.6"/>
        <path d="M12.8 1H10.4L8 5L5.6 1H3.2L8 9.5L12.8 1Z" fill="#35495E"/>
    </svg>
{:else if framework === 'nextjs'}
    <svg class={className} viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg">
        <circle cx="8" cy="8" r="7" fill="currentColor"/>
        <path d="M6 5V11L11.5 5" stroke="var(--background, white)" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/>
        <path d="M10.5 5V11" stroke="var(--background, white)" stroke-width="1.2" stroke-linecap="round"/>
    </svg>
{:else if framework === 'nestjs'}
    <svg class={className} viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg">
        <path d="M9.2 1.2C8.9 1.2 8.6 1.3 8.4 1.5C9 1.7 9.4 2.2 9.5 2.8C9.5 2.9 9.5 3 9.5 3.1C9.5 3.2 9.5 3.3 9.4 3.4C9.4 3.7 9.5 4 9.7 4.2C10 4.5 10.4 4.6 10.7 4.5C10.7 4.1 10.9 3.7 11.2 3.4C11.1 2.1 10.3 1.2 9.2 1.2Z" fill="#E0234E"/>
        <path d="M14 6.5C13.7 5.5 13.1 4.6 12.3 4C12.3 4 12.2 4 12.2 3.9C11.9 3.8 11.5 3.9 11.3 4.1C11.1 4.3 11 4.6 11.1 4.9C11.1 5 11.2 5.1 11.2 5.2C11.3 5.3 11.3 5.3 11.3 5.4C11 5.2 10.6 5.1 10.2 5.1C9.8 5.1 9.5 5.2 9.2 5.3C8 5.8 7.2 5.5 6.6 5.5C5.8 5.4 5.1 5.7 4.5 6.2C3.5 7.1 3.1 8.5 3.4 9.7C3.7 11.3 4.5 12.8 5.7 13.8C6.2 14.3 6.9 14.6 7.6 14.6C8 14.6 8.3 14.5 8.6 14.3C9.2 14.6 9.9 14.7 10.5 14.5C11.3 14.2 11.8 13.5 12 12.7C12.5 12.4 13 12 13.3 11.5C14.1 10 14.4 8.2 14 6.5Z" fill="#E0234E"/>
    </svg>
{:else if framework === 'express'}
    <svg class={className} viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg">
        <path d="M2 4H14V5.5H5V7.2H13V8.7H5V10.5H14V12H2V4Z" fill="currentColor"/>
    </svg>
{:else if framework === 'remix'}
    <svg class={className} viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg">
        <path d="M13.5 12.5H9.8C9.8 11.5 9.8 11 9.2 10.3C8.6 9.6 7.7 9.5 7 9.5H2.5V12.5H2.5V14H13.5V12.5Z" fill="currentColor"/>
        <path d="M7.5 2C10.5 2 13 3.5 13 6C13 7.8 11.8 9 9.5 9.3C10.6 9.6 11.2 10.3 11.3 11.5H13.5V10H15V13H13.5V12.5H9.8C9.8 11.2 9.4 10.5 7.5 10.5H2.5V8H7.5C9.8 8 11 7.2 11 5.8C11 4.4 9.8 3.5 7.5 3.5H2.5V2H7.5Z" fill="currentColor"/>
    </svg>
{:else}
    <img src="/custom.png" alt="Custom" class={className} />
{/if}
