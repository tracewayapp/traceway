import { afterEach, expect, it, vi } from 'vitest';
import { cleanup, render, screen } from '@testing-library/svelte';

vi.mock('$lib/state/projects.svelte', () => ({
	projectsState: { currentProjectId: 'project-1', currentProject: { framework: 'android' } }
}));

import SessionRecordingReasons from './session-recording-reasons.svelte';

afterEach(() => cleanup());

// TW-3: ClickHouse can return a trigger revision twice, labelled before and after the server renamed its steps.
it('shows one chip per trigger revision', () => {
	render(SessionRecordingReasons, {
		reasons: [
			{
				triggerId: 'default-exception',
				revision: 'r1',
				name: 'default-exception',
				sequence: 'event'
			},
			{
				triggerId: 'default-exception',
				revision: 'r1',
				name: 'default-exception',
				sequence: 'exception'
			},
			{ triggerId: 'journey', revision: 'r2', name: 'Checkout', sequence: 'click(#pay)' }
		]
	});
	expect(screen.getAllByRole('link').map((link) => link.textContent?.trim())).toEqual([
		'event',
		'Checkout'
	]);
});
