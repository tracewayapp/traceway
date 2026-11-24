declare module '$billing/cross-site-notification-baner.svelte' {
	import type { Component } from 'svelte';
	const component: Component<{ organizationId: number }>;
	export default component;
}

// Mounted once in the signed-in app shell (and in the setup flow), outside the sidebar, for UI that
// floats over every page: Traceway Cloud's help button and its panel.
declare module '$billing/overlay-slot.svelte' {
	import type { Component } from 'svelte';
	const component: Component;
	export default component;
}
