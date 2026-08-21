import { goto } from '$app/navigation';
import { resolve } from '$app/paths';

import { splitHref } from './links';

export { authenticatedLandingPath, defaultAuthenticatedPath, safeLocalPath } from './landing';

export function gotoHref(href: string, options?: Parameters<typeof goto>[1]) {
	const { pathname, suffix } = splitHref(href);
	let destination = resolve((pathname || '/') as '/');
	destination += suffix;
	return goto(destination, options);
}

// SSO logins bounce through the provider and land on /auth/callback, losing
// any ?returnTo= the login page had. Stash it in sessionStorage before leaving
// and consume it when the callback (or finish-setup) decides where to land.
const SSO_RETURN_TO_KEY = 'traceway_sso_return_to';

export function stashSsoReturnTo(path: string | null) {
	if (path) {
		sessionStorage.setItem(SSO_RETURN_TO_KEY, path);
	} else {
		sessionStorage.removeItem(SSO_RETURN_TO_KEY);
	}
}

export function consumeSsoReturnTo(): string | null {
	const path = sessionStorage.getItem(SSO_RETURN_TO_KEY);
	sessionStorage.removeItem(SSO_RETURN_TO_KEY);
	return path;
}

export function addStickyParamsToHref(href: string, ...stickyParams: string[]) {
	const currentParams = new URLSearchParams(window.location.search);
	const url = new URL(href, window.location.origin);

	// projectId is the source of truth for the selected project, so it is always
	// sticky. An explicit projectId already in href (e.g. a cross-project link)
	// wins over the current one.
	if (!url.searchParams.has('projectId')) {
		const currentProjectId = currentParams.get('projectId');
		if (currentProjectId !== null) {
			url.searchParams.set('projectId', currentProjectId);
		}
	}

	stickyParams.forEach((stickyParam) => {
		if (stickyParam === 'projectId') return;
		const currentValue = currentParams.get(stickyParam);
		if (currentValue !== null) {
			url.searchParams.set(stickyParam, currentValue);
		}
	});

	return url.pathname + url.search;
}

// in the future it would be really cool if we could bind the type here to get type safety and force the use of resolve :/
// this also won't work with absolute paths - meh so be it
export function createRowClickHandlerWithNavigate(
	href: string,
	onBeforeNavigate: (() => void) | undefined,
	...stickyParams: string[]
) {
	return (event: MouseEvent) => {
		onBeforeNavigate?.();

		const finalHref = addStickyParamsToHref(href, ...stickyParams);

		if (event.ctrlKey || event.metaKey) {
			window.open(finalHref, '_blank');
		} else {
			gotoHref(finalHref);
		}
	};
}

export function createRowClickHandler(href: string, ...stickyParams: string[]) {
	return createRowClickHandlerWithNavigate(href, undefined, ...stickyParams);
}
