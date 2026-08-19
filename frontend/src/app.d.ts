// See https://svelte.dev/docs/kit/types#app.d.ts
// for information about these interfaces
declare global {
	namespace App {
		// interface Error {}
		// interface Locals {}
		// interface PageData {}
		// interface PageState {}
		// interface Platform {}
	}

	const __APP_VERSION__: string;
	const __CLOUD_MODE__: string;
	const __TURNSTILE_SITE_KEY__: string;
	const __TRACEWAY_URL__: string;

	interface Window {
		captureException: typeof import('@tracewayapp/frontend').captureException;
		turnstile: {
			render: (
				element: HTMLElement,
				options: {
					sitekey: string;
					callback: (token: string) => void;
					'error-callback'?: () => void;
					'expired-callback'?: () => void;
					theme?: 'light' | 'dark' | 'auto';
				}
			) => string;
			remove: (widgetId: string) => void;
			reset: (widgetId: string) => void;
		};
		onTurnstileLoad?: () => void;
	}
}

export {};
