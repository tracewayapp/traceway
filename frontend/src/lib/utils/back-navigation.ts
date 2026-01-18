import { goto } from '$app/navigation';

const NAV_DEPTH_KEY = 'traceway_nav_depth';

export interface SmartBackOptions {
	fallbackPath: string;
}

/**
 * Get current navigation depth
 */
function getNavDepth(): number {
	try {
		return parseInt(sessionStorage.getItem(NAV_DEPTH_KEY) || '0', 10);
	} catch {
		return 0;
	}
}

/**
 * Set navigation depth
 */
function setNavDepth(depth: number): void {
	try {
		sessionStorage.setItem(NAV_DEPTH_KEY, String(depth));
	} catch {
		// Ignore storage errors
	}
}

/**
 * Called when navigating to a new page (pathname changed)
 */
export function incrementNavDepth(): void {
	setNavDepth(getNavDepth() + 1);
}

/**
 * Called on popstate (browser back/forward)
 */
export function decrementNavDepth(): void {
	setNavDepth(Math.max(0, getNavDepth() - 1));
}

/**
 * Clear navigation depth (call on logout)
 */
export function clearNavDepth(): void {
	try {
		sessionStorage.removeItem(NAV_DEPTH_KEY);
	} catch {
		// Ignore storage errors
	}
}

/**
 * Check if there's internal history to go back to
 */
export function hasInternalHistory(): boolean {
	return getNavDepth() > 0;
}

/**
 * Create a smart back button handler that:
 * 1. Uses history.back() if there's valid internal navigation history
 * 2. Falls back to the specified path if no history (direct URL access, refresh with no prior nav)
 */
export function createSmartBackHandler(options: SmartBackOptions): (e: MouseEvent) => void {
	const { fallbackPath } = options;

	return (event: MouseEvent) => {
		// Ctrl/Cmd+Click: open fallback in new tab
		if (event.ctrlKey || event.metaKey) {
			window.open(fallbackPath, '_blank');
			return;
		}

		if (hasInternalHistory()) {
			history.back();
		} else {
			goto(fallbackPath);
		}
	};
}
