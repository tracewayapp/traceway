import { resolve } from '$app/paths';

export function splitHref(href: string): { pathname: string; suffix: string } {
	const suffixIndex = href.search(/[?#]/);
	if (suffixIndex === -1) return { pathname: href, suffix: '' };
	return { pathname: href.slice(0, suffixIndex), suffix: href.slice(suffixIndex) };
}

export function resolveHref(href: string): string {
	if (/^(?:[a-z][a-z\d+.-]*:)?\/\//i.test(href) || href.startsWith('#')) return href;
	const { pathname, suffix } = splitHref(href);
	return resolve((pathname || '/') as '/') + suffix;
}
