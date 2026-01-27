<script lang="ts">
    import { onMount, onDestroy } from 'svelte';
    import { themeState } from '$lib/state/theme.svelte';

    interface Props {
        siteKey: string;
        onVerify: (token: string) => void;
        onError?: () => void;
    }

    let { siteKey, onVerify, onError }: Props = $props();

    let container: HTMLDivElement;
    let widgetId: string | null = null;

    declare global {
        interface Window {
            turnstile: {
                render: (element: HTMLElement, options: {
                    sitekey: string;
                    callback: (token: string) => void;
                    'error-callback'?: () => void;
                    'expired-callback'?: () => void;
                    theme?: 'light' | 'dark' | 'auto';
                }) => string;
                remove: (widgetId: string) => void;
            };
            onTurnstileLoad?: () => void;
        }
    }

    function loadScript(): Promise<void> {
        return new Promise((resolve) => {
            if (window.turnstile) {
                resolve();
                return;
            }

            const existingScript = document.querySelector('script[src*="turnstile"]');
            if (existingScript) {
                window.onTurnstileLoad = resolve;
                return;
            }

            window.onTurnstileLoad = resolve;
            const script = document.createElement('script');
            script.src = 'https://challenges.cloudflare.com/turnstile/v0/api.js?onload=onTurnstileLoad';
            script.async = true;
            script.defer = true;
            document.head.appendChild(script);
        });
    }

    function renderWidget() {
        if (!window.turnstile || !container) return;

        widgetId = window.turnstile.render(container, {
            sitekey: siteKey,
            callback: onVerify,
            'error-callback': onError,
            'expired-callback': () => {
                onVerify('');
            },
            theme: themeState.isDark ? 'dark' : 'light'
        });
    }

    onMount(async () => {
        await loadScript();
        renderWidget();
    });

    onDestroy(() => {
        if (widgetId && window.turnstile) {
            window.turnstile.remove(widgetId);
        }
    });
</script>

<div bind:this={container} class="flex justify-center"></div>
