import { describe, expect, it } from 'vitest';
import { getSetupPrompt, getSetupPromptParts, uploadTokenLabel } from './ai-setup';

const BACKEND = 'https://cloud.tracewayapp.com';

describe('the AI setup prompt', () => {
	it('names the project framework', () => {
		expect(getSetupPrompt(BACKEND, 'tok', null, 'react')).toBe(
			'/traceway-setup for a React project with token tok and url https://cloud.tracewayapp.com'
		);
		expect(getSetupPrompt(BACKEND, 'tok', null, 'ios')).toBe(
			'/traceway-setup for an iOS project with token tok and url https://cloud.tracewayapp.com'
		);
		expect(getSetupPrompt(BACKEND, 'tok', null, 'opentelemetry')).toContain(
			'for an OpenTelemetry project'
		);
		const parts = getSetupPromptParts(BACKEND, 'tok', null, 'android');
		expect(parts.find((p) => p.text === 'Android')?.bold).toBe(true);
	});

	it('names the upload token for what it uploads', () => {
		expect(getSetupPrompt(BACKEND, 'tok', 'up', 'svelte')).toBe(
			'/traceway-setup for a Svelte project with token tok and url https://cloud.tracewayapp.com and source map upload token up'
		);
		expect(getSetupPrompt(BACKEND, 'tok', 'up', 'flutter')).toContain(
			' and symbol upload token up'
		);
		expect(uploadTokenLabel('android')).toBe('symbol upload token');
		expect(uploadTokenLabel('jquery')).toBe('source map upload token');
	});

	it('still works without a framework', () => {
		expect(getSetupPrompt(BACKEND, 'tok')).toBe(
			'/traceway-setup with token tok and url https://cloud.tracewayapp.com'
		);
	});
});
