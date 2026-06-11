import { browser } from '$app/environment';
import { OTEL_TARGETS, type OtelTargetId } from './otel-setup';

export type SetupMode = 'ai' | 'manual';

const MODE_KEY = 'traceway_setup_mode';
const OTEL_TARGET_KEY = 'traceway_otel_language';
const OTEL_FRAMEWORK_KEY = 'traceway_otel_framework';

export function getSetupMode(): SetupMode {
	if (!browser) return 'ai';

	try {
		const stored = localStorage.getItem(MODE_KEY);
		if (stored === 'ai' || stored === 'manual') {
			return stored;
		}
	} catch {}

	return 'ai';
}

export function setSetupMode(mode: SetupMode): void {
	if (!browser) return;

	try {
		localStorage.setItem(MODE_KEY, mode);
	} catch {}
}

export function getOtelTarget(): OtelTargetId {
	if (!browser) return OTEL_TARGETS[0].id;

	try {
		const stored = localStorage.getItem(OTEL_TARGET_KEY);
		if (stored && OTEL_TARGETS.some((t) => t.id === stored)) {
			return stored as OtelTargetId;
		}
	} catch {}

	return OTEL_TARGETS[0].id;
}

export function setOtelTarget(id: OtelTargetId): void {
	if (!browser) return;

	try {
		localStorage.setItem(OTEL_TARGET_KEY, id);
	} catch {}
}

export function getOtelFramework(): string | null {
	if (!browser) return null;

	try {
		return localStorage.getItem(OTEL_FRAMEWORK_KEY);
	} catch {}

	return null;
}

export function setOtelFramework(id: string): void {
	if (!browser) return;

	try {
		localStorage.setItem(OTEL_FRAMEWORK_KEY, id);
	} catch {}
}
