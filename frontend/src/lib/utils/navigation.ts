import { goto } from '$app/navigation';

// in the future it would be really cool if we could bind the type here to get type safety and force the use of resolve :/
// this also won't work with absolute paths - meh so be it
export function createRowClickHandler(href: string, ...stickyParams: string[]) {
	return (event: MouseEvent) => {
		let finalHref = href;

		if (stickyParams.length > 0) {
			const currentParams = new URLSearchParams(window.location.search);
			const url = new URL(href, window.location.origin);

			stickyParams.forEach(stickyParam => {
				const currentValue = currentParams.get(stickyParam);
				if (currentValue !== null) {
					url.searchParams.set(stickyParam, currentValue);
				}
			});

			finalHref = url.pathname + url.search;
		}

		if (event.ctrlKey || event.metaKey) {
			window.open(finalHref, '_blank');
		} else {
			// eslint-disable-next-line svelte/no-navigation-without-resolve
			goto(finalHref);
		}
	};
}