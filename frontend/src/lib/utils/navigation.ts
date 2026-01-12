import { goto } from '$app/navigation';

/**
 * Handles row click with support for ctrl/cmd+click to open in new tab.
 * Returns an onclick handler for use in table rows.
 */
export function createRowClickHandler(href: string) {
	return (event: MouseEvent) => {
		// Check for ctrl (Windows/Linux) or cmd (Mac) key
		if (event.ctrlKey || event.metaKey) {
			// Open in new tab
			window.open(href, '_blank');
		} else {
			// Normal navigation
			goto(href);
		}
	};
}
