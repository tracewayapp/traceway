import { formatDuration } from './formatters';

export const SESSION_IDLE_TIMEOUT_MS = 15 * 60_000;

export type SessionTiming = {
	startedAt: string;
	endedAt?: string | null;
	duration: number;
	hasRecording?: boolean;
};

export function sessionDurationLabel(session: SessionTiming, now = Date.now()): string {
	if (session.endedAt) return formatDuration(session.duration);
	if (session.hasRecording) return 'in progress';
	const startedMs = Date.parse(session.startedAt);
	if (Number.isFinite(startedMs) && now - startedMs < SESSION_IDLE_TIMEOUT_MS) {
		return 'in progress';
	}
	return 'No recording';
}
