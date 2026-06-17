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
{:else if framework === 'react' || framework === 'react-native'}
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
{:else if framework === 'jquery'}
    <img src="/jquery.png" alt="jQuery" class={className} />
{:else if framework === 'hono'}
    <img src="/hono.png" alt="Hono" class={className} />
{:else if framework === 'cloudflare'}
    <img src="/cloudflare.png" alt="Cloudflare" class={className} />
{:else if framework === 'opentelemetry'}
    <img src="/opentelemetry.png" alt="OpenTelemetry" class={className} />
{:else if framework === 'symfony'}
    <img src="/symfony.png" alt="Symfony" class={className} />
{:else if framework === 'laravel'}
    <img src="/laravel.png" alt="Laravel" class={className} />
{:else if framework === 'django'}
    <img src="/django.png" alt="Django" class={className} style="object-fit: contain;" />
{:else if framework === 'flutter'}
    <img src="/flutter.png" alt="Flutter" class={className} style="object-fit: contain;" />
{:else if framework === 'android'}
    <img src="/android.png" alt="Android" class={className} style="object-fit: contain;" />
{:else if framework === 'ios'}
    <svg class={className} viewBox="0 1 16 16" fill="currentColor" xmlns="http://www.w3.org/2000/svg">
        <path d="M11.05 8.5c-.02-1.5 1.23-2.22 1.28-2.26-.7-1.02-1.78-1.16-2.17-1.18-.92-.09-1.8.54-2.27.54-.47 0-1.19-.53-1.96-.51-1.01.01-1.94.59-2.46 1.49-1.05 1.82-.27 4.51.76 5.99.5.72 1.1 1.53 1.88 1.5.76-.03 1.04-.49 1.96-.49.91 0 1.17.49 1.97.47.81-.01 1.33-.73 1.83-1.46.58-.84.81-1.65.83-1.69-.02-.01-1.59-.61-1.61-2.43zM9.6 4.07c.42-.51.7-1.21.62-1.91-.6.02-1.33.4-1.76.91-.39.45-.73 1.17-.64 1.85.67.05 1.36-.34 1.78-.85z"/>
    </svg>
{:else}
    <img src="/custom.png" alt="Custom" class={className} />
{/if}
