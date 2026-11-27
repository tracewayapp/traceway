// The replay-image values of a session's Overview, from the attributes of its session events
// (`traceway.replay.images`, the policy the SDK applied; `traceway.replay.image_bytes`, the image data it sent).
import { REPLAY_IMAGES_LABELS, type ReplayImages } from '$lib/components/sdk-config/sdk-config';
import { formatMetricValue } from './metric-format';

export type SessionAttributes = Record<string, string> | null | undefined;

/** "Wi‑Fi only", "Always" or "Off"; an unknown value as sent; null when the session did not report one. */
export function replayImagesLabel(attributes: SessionAttributes): string | null {
	const value = attributes?.['traceway.replay.images']?.trim();
	if (!value) return null;
	return REPLAY_IMAGES_LABELS[value as ReplayImages] ?? value;
}

/** "1.2 MB", "512 B"; null when the session did not report it or it is not a number. */
export function replayImageDataLabel(attributes: SessionAttributes): string | null {
	const raw = attributes?.['traceway.replay.image_bytes']?.trim();
	if (!raw) return null;
	const bytes = Number(raw);
	if (!Number.isFinite(bytes) || bytes < 0) return null;
	const { text, unit } = formatMetricValue(bytes, 'B');
	return `${text} ${unit}`;
}
