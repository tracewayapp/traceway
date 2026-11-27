declare module '$billing/cross-site-notification-baner.svelte' {
	import type { Component } from 'svelte';
	const component: Component<{ organizationId: number }>;
	export default component;
}

// Mounted once in the signed-in app shell (and in the setup flow), outside the sidebar, for UI that
// floats over every signed-in page.
declare module '$billing/overlay-slot.svelte' {
	import type { Component } from 'svelte';
	const component: Component;
	export default component;
}

// Mounted inside the two forms that create an account's first organization (register and SSO
// finish-setup), above the submit button, for an extension's own optional fields. Core sends none
// of it: the extension carries what it collects to its own hooks, for example in a cookie.
declare module '$billing/signup-slot.svelte' {
	import type { Component } from 'svelte';
	const component: Component;
	export default component;
}

// Mounted once on every dashboard page, signed in or not (never on a status domain or the embed
// page), for extension code that has to run before sign-in.
declare module '$billing/global-slot.svelte' {
	import type { Component } from 'svelte';
	const component: Component;
	export default component;
}
