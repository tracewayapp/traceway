/**
 * The issue types the app SDKs read from the OS about a process: a native crash, an ANR, a
 * main-thread hang and an iOS watchdog kill. The server groups them by their top in-app frames.
 */
export const APP_ISSUE_TYPES = [
	'NativeCrash',
	'ApplicationNotResponding',
	'AppHang',
	'WatchdogTermination'
] as const;

export type AppIssueType = (typeof APP_ISSUE_TYPES)[number];

function text(value: unknown): string {
	return typeof value === 'string' ? value.trim() : '';
}

/** The app issue type a trace's first line names (`AppHang: Main thread blocked for 3.2 s`), or null. */
export function appIssueType(stackTrace: unknown): AppIssueType | null {
	if (typeof stackTrace !== 'string') return null;
	const type = stackTrace.split('\n', 1)[0].split(':', 1)[0].trim();
	return (APP_ISSUE_TYPES as readonly string[]).includes(type) ? (type as AppIssueType) : null;
}

const UNIT_MS: Record<string, number> = { ms: 1, s: 1000, sec: 1000, min: 60_000 };

/** How long a hang blocked the main thread, from its message: `Main thread blocked for 3.2 s`. */
export function blockedMs(message: unknown): number | null {
	const m = text(message).match(
		/\bblocked for\s+(?:at least\s+)?(\d+(?:\.\d+)?)\s*(ms|s|sec|seconds?|min|minutes?)\b/i
	);
	if (!m) return null;
	const unit = m[2].toLowerCase().replace(/(?:utes?|onds?)$/, '');
	const ms = Number(m[1]) * (UNIT_MS[unit] ?? 1000);
	return Number.isFinite(ms) ? ms : null;
}

export type HangDetails = {
	/** How long the main thread was blocked, when the message says. */
	durationMs: number | null;
	/** The process did not survive the hang; the SDK reported it at the next launch (`app.hang.fatal`). */
	fatal: boolean;
};

/** What an AppHang occurrence says about its hang, or null for any other issue. */
export function hangDetails(
	stackTrace: unknown,
	attributes: Record<string, string> | null | undefined
): HangDetails | null {
	if (appIssueType(stackTrace) !== 'AppHang') return null;
	const firstLine = (stackTrace as string).split('\n', 1)[0];
	return {
		durationMs: blockedMs(firstLine) ?? blockedMs(attributes?.['exception.message']),
		fatal: text(attributes?.['app.hang.fatal']).toLowerCase() === 'true'
	};
}

/** A span of time to read at a glance: `850 ms`, `3.2 s`, `4 min 12 s`, `1 h 5 min`. */
export function formatElapsed(ms: number): string {
	if (!Number.isFinite(ms) || ms < 0) return '-';
	if (ms < 999.5) return `${Math.round(ms)} ms`;
	if (ms < 9_950) return `${Number((ms / 1000).toFixed(1))} s`;
	const seconds = Math.round(ms / 1000);
	if (seconds < 60) return `${seconds} s`;
	if (seconds < 3600) {
		const s = seconds % 60;
		return s > 0 ? `${Math.floor(seconds / 60)} min ${s} s` : `${seconds / 60} min`;
	}
	const minutes = Math.round(ms / 60_000);
	const min = minutes % 60;
	return min > 0 ? `${Math.floor(minutes / 60)} h ${min} min` : `${minutes / 60} h`;
}

/**
 * Where the app or page was when something was recorded, as every SDK stamps it: the screen (the
 * route template set with setRoute, else the path), its route template, the screen before it, how long it had been
 * showing and the window's size.
 */
export type ScreenContext = {
	name?: string;
	route?: string;
	previous?: string;
	durationMs?: number;
	viewport?: string;
};

export function screenContext(
	attributes: Record<string, string> | null | undefined
): ScreenContext | null {
	if (!attributes || typeof attributes !== 'object') return null;
	const out: ScreenContext = {};
	const name = text(attributes['app.screen.name']);
	const route = text(attributes['app.screen.route']);
	const previous = text(attributes['app.screen.previous']);
	const viewport = text(attributes['viewport']);
	const duration = Number(text(attributes['app.screen.duration_ms']) || NaN);
	if (name) out.name = name;
	if (route) out.route = route;
	if (previous) out.previous = previous;
	if (Number.isFinite(duration) && duration >= 0) out.durationMs = duration;
	if (viewport) out.viewport = viewport;
	return Object.keys(out).length > 0 ? out : null;
}

/** `411x914` as `411 × 914`; anything else as it came. */
export function formatViewport(viewport: string): string {
	const m = viewport.match(/^\s*(\d+)\s*[x×]\s*(\d+)\s*$/i);
	return m ? `${m[1]} × ${m[2]}` : viewport;
}
