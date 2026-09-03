import { base } from '$app/paths';

function splitHref(href: string): { pathname: string; suffix: string } {
	const suffixIndex = href.search(/[?#]/);
	if (suffixIndex === -1) return { pathname: href, suffix: '' };
	return { pathname: href.slice(0, suffixIndex), suffix: href.slice(suffixIndex) };
}

// Not resolve() from $app/paths: that reads its argument as a route id and
// throws on the literal brackets a concrete pathname can carry. Prefixing the
// base path is what a concrete href needs instead.
export function resolveHref(href: string): string {
	if (/^(?:[a-z][a-z\d+.-]*:|\/\/|#)/i.test(href)) return href;
	const { pathname, suffix } = splitHref(href);
	return base + (pathname || '/') + suffix;
}
