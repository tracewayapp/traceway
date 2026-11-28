import { cleanup, render, screen } from '@testing-library/svelte';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { projectsState, type Project } from '$lib/state/projects.svelte';
import type { SetupDraft, SetupPlanProject } from '$lib/utils/setup-plan';
import SetupProjectsStep from './setup-projects-step.svelte';

const { get, post, gotoHref } = vi.hoisted(() => ({
	get: vi.fn(),
	post: vi.fn(),
	gotoHref: vi.fn()
}));
vi.mock('$lib/api', () => ({ api: { get, post } }));
vi.mock('$lib/utils/navigation', async (importOriginal) => ({
	...(await importOriginal<typeof import('$lib/utils/navigation')>()),
	gotoHref
}));

const ORG = 5;

function project(id: string, organizationId = ORG): Project {
	return {
		id,
		name: 'Project ' + id,
		token: 'token-' + id,
		framework: 'android',
		organizationId,
		createdAt: '2026-11-27T10:00:00Z',
		sourceMapToken: 'upload-' + id,
		backendUrl: 'https://cloud.tracewayapp.com',
		dropHealthyHealthchecks: false,
		healthcheckPaths: null,
		profileLabelAllowlist: null,
		aiFlaggedTerms: null,
		aiFlaggedLanguages: null
	};
}

function draft(status: SetupDraft['status'], planned: SetupPlanProject, created: Project[]) {
	return {
		id: 'draft-1',
		status,
		payload: { projects: [planned] },
		createdAt: '2026-11-27T10:00:00Z',
		requestedByEmail: 'dev@example.com',
		projects: created.map((p) => ({
			id: p.id,
			name: planned.name,
			framework: planned.framework,
			token: p.token,
			sourceMapToken: p.sourceMapToken,
			backendUrl: p.backendUrl,
			status: 'created' as const
		}))
	} satisfies SetupDraft;
}

// What the server answers: the proposal, and the project list of each load in turn (the last one repeats).
function server(proposal: SetupDraft | null, ...loads: Project[][]) {
	get.mockResolvedValue({ draft: proposal });
	post.mockResolvedValue(null);
	let call = 0;
	vi.spyOn(projectsState, 'loadProjects').mockImplementation(async () => {
		projectsState.setProjects(loads[Math.min(call++, loads.length - 1)]);
	});
}

const mount = () => render(SetupProjectsStep, { organizationId: ORG, initialMode: 'ai' });
const continueButton = () => screen.queryByRole('button', { name: /Continue to Dashboard/ });

beforeEach(() => {
	vi.useFakeTimers();
	vi.stubGlobal('matchMedia', () => ({ matches: false }));
	projectsState.setProjects([]);
});

afterEach(() => {
	cleanup();
	vi.useRealTimers();
	vi.unstubAllGlobals();
	vi.restoreAllMocks();
	vi.clearAllMocks();
});

it('stays on the upload tokens of an approved plan that left them to the page', async () => {
	const created = project('p1');
	server(
		draft('approved', { name: 'Shop Android', framework: 'android' }, [created]),
		[],
		[created]
	);
	mount();
	await vi.advanceTimersByTimeAsync(3000 + 2500);

	expect(screen.getByText('Next Steps: Upload Tokens')).toBeTruthy();
	expect(gotoHref).not.toHaveBeenCalled();
	expect(continueButton()).not.toBeNull();
});

it('moves on by itself when the approved plan leaves nothing to copy', async () => {
	const created = project('p1');
	const planned = { name: 'Shop Android', framework: 'android', uploadTokenEnvFile: '.env' };
	server(draft('approved', planned, [created]), [], [created]);
	mount();
	await vi.advanceTimersByTimeAsync(3000 + 2500);

	expect(screen.queryByText('Next Steps: Upload Tokens')).toBeNull();
	expect(gotoHref).toHaveBeenCalledWith('/?projectId=p1', { replaceState: true });
});

it('does not take a project missing from a stale cache for one this setup created', async () => {
	const cached = project('p1');
	const teammates = project('p2');
	projectsState.setProjects([cached]);
	server(draft('pending', { name: 'Shop iOS', framework: 'ios' }, []), [cached, teammates]);
	mount();
	await vi.advanceTimersByTimeAsync(3000 + 2500);

	expect(screen.getByRole('button', { name: /Approve/ })).toBeTruthy();
	expect(gotoHref).not.toHaveBeenCalled();
	expect(screen.queryByText('Created')).toBeNull();
});
