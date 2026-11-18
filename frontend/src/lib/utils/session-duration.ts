export const SESSION_IDLE_TIMEOUT_MS = 15 * 60_000;

export type SessionTiming = {
	startedAt: string;
	endedAt?: string | null;
	duration: number;
	hasRecording?: boolean;
};

export function formatSessionDuration(nanoseconds: number): string {
	const totalSeconds = Math.max(0, nanoseconds) / 1e9;
	if (totalSeconds < 60) return `${totalSeconds.toFixed(1)}s`;
	const seconds = Math.floor(totalSeconds);
	const hours = Math.floor(seconds / 3600);
	const minutes = Math.floor((seconds % 3600) / 60);
	if (hours > 0) return `${hours}h ${minutes}m`;
	return `${minutes}m ${seconds % 60}s`;
}

export function sessionDurationLabel(session: SessionTiming, now = Date.now()): string {
	if (session.endedAt) return formatSessionDuration(session.duration);
	if (session.hasRecording) return 'in progress';
	const startedMs = Date.parse(session.startedAt);
	if (Number.isFinite(startedMs) && now - startedMs < SESSION_IDLE_TIMEOUT_MS) {
		return 'in progress';
	}
	return 'No recording';
}
