import { goto } from '$app/navigation';

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

	stickyParams.forEach(stickyParam => {
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
export function createRowClickHandlerWithNavigate(href: string, onBeforeNavigate: (() => void) | undefined, ...stickyParams: string[]) {
	return (event: MouseEvent) => {
		onBeforeNavigate?.();

		const finalHref = addStickyParamsToHref(href, ...stickyParams);

		if (event.ctrlKey || event.metaKey) {
			window.open(finalHref, '_blank');
		} else {
			// eslint-disable-next-line svelte/no-navigation-without-resolve
			goto(finalHref);
		}
	};
}

export function createRowClickHandler(href: string, ...stickyParams: string[]) {
	return createRowClickHandlerWithNavigate(href, undefined, ...stickyParams);
}