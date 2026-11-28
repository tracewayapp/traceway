import { describe, expect, it } from 'vitest';
import { replayImageDataLabel, replayImagesLabel } from './session-overview';

describe('replayImagesLabel', () => {
	it('names the policy the SDK applied', () => {
		expect(replayImagesLabel({ 'traceway.replay.images': 'wifi' })).toBe('Wi‑Fi only');
		expect(replayImagesLabel({ 'traceway.replay.images': 'always' })).toBe('Always');
		expect(replayImagesLabel({ 'traceway.replay.images': 'off' })).toBe('Off');
	});

	it('shows an unknown value as sent', () => {
		expect(replayImagesLabel({ 'traceway.replay.images': 'metered' })).toBe('metered');
	});

	it('is null when the session did not report it', () => {
		expect(replayImagesLabel(null)).toBeNull();
		expect(replayImagesLabel(undefined)).toBeNull();
		expect(replayImagesLabel({})).toBeNull();
		expect(replayImagesLabel({ 'traceway.replay.images': '' })).toBeNull();
	});
});

describe('replayImageDataLabel', () => {
	it('formats the image bytes the session sent', () => {
		expect(replayImageDataLabel({ 'traceway.replay.image_bytes': '1258291' })).toBe('1.2 MB');
		expect(replayImageDataLabel({ 'traceway.replay.image_bytes': '49152' })).toBe('48.0 KB');
		expect(replayImageDataLabel({ 'traceway.replay.image_bytes': '512' })).toBe('512 B');
		expect(replayImageDataLabel({ 'traceway.replay.image_bytes': '0' })).toBe('0 B');
	});

	it('is null when the session did not report it or it is not a number', () => {
		expect(replayImageDataLabel(null)).toBeNull();
		expect(replayImageDataLabel({})).toBeNull();
		expect(replayImageDataLabel({ 'traceway.replay.image_bytes': '' })).toBeNull();
		expect(replayImageDataLabel({ 'traceway.replay.image_bytes': 'lots' })).toBeNull();
		expect(replayImageDataLabel({ 'traceway.replay.image_bytes': '-1' })).toBeNull();
	});
});
