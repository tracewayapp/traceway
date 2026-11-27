import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';

const state = vi.hoisted(() => ({ framework: 'android' }));
const post = vi.hoisted(() => vi.fn().mockResolvedValue({}));
const put = vi.hoisted(() => vi.fn().mockResolvedValue({}));

vi.mock('$lib/api', () => ({ api: { get: vi.fn().mockResolvedValue(null), post, put } }));
vi.mock('$lib/state/projects.svelte', async (importOriginal) => ({
	...(await importOriginal<typeof import('$lib/state/projects.svelte')>()),
	projectsState: {
		currentProjectId: 'project-1',
		get currentProject() {
			return { id: 'project-1', framework: state.framework };
		},
		canWriteCurrentProject: true
	}
}));

import RuleDialog from './sdk-config-rule-dialog.svelte';
import LogTriggerBuilder from './log-trigger-builder.svelte';
import type { SdkConfigRule } from './sdk-config';

beforeEach(() => {
	state.framework = 'android';
	post.mockClear();
	put.mockClear();
});

afterEach(() => cleanup());

const nameInput = () => document.getElementById('sdk-rule-name') as HTMLInputElement;
const activeTab = () => screen.getAllByRole('tab').find((tab) => tab.dataset.state === 'active');
const tab = (name: RegExp) => screen.getByRole('tab', { name });

function conditionInputs() {
	return {
		keys: screen.queryAllByPlaceholderText('user.id') as HTMLInputElement[],
		values: screen.queryAllByPlaceholderText('value') as HTMLInputElement[]
	};
}

function expectConditionKept() {
	const { keys, values } = conditionInputs();
	expect(keys.map((input) => input.value)).toEqual(['user.id']);
	expect(values.map((input) => input.value)).toEqual(['u-42']);
}

describe('rule dialog', () => {
	it('keeps a new rule and its tab when switching to JSON and adding a log trigger', async () => {
		render(RuleDialog, { open: true, rule: null, onSaved: vi.fn() });

		await fireEvent.input(nameInput(), { target: { value: 'Checkout journey' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Add condition' }));
		await fireEvent.input(conditionInputs().keys[0], { target: { value: 'user.id' } });
		await fireEvent.input(conditionInputs().values[0], { target: { value: 'u-42' } });

		await fireEvent.click(tab(/^JSON/));
		expect(activeTab()?.textContent?.trim()).toBe('JSON');
		expect(nameInput().value).toBe('Checkout journey');

		await fireEvent.click(tab(/^Applies to/));
		expect(activeTab()?.textContent?.trim()).toBe('Applies to (1)');
		expect(nameInput().value).toBe('Checkout journey');
		expectConditionKept();

		await fireEvent.click(tab(/^Triggers/));
		expect(activeTab()?.textContent?.trim()).toBe('Triggers');
		await fireEvent.input(screen.getByPlaceholderText('payment failed'), {
			target: { value: 'Payment declined' }
		});
		await fireEvent.click(screen.getByRole('button', { name: 'Add trigger' }));

		// Adding a log trigger opens the JSON tab with the trigger in it.
		expect(activeTab()?.textContent?.trim()).toBe('JSON');
		expect(tab(/^Triggers/).textContent?.trim()).toBe('Triggers (1)');
		expect(nameInput().value).toBe('Checkout journey');

		await fireEvent.click(tab(/^Applies to/));
		expect(activeTab()?.textContent?.trim()).toBe('Applies to (1)');
		expectConditionKept();

		await fireEvent.click(screen.getByRole('button', { name: 'New Rule' }));
		expect(post).toHaveBeenCalledTimes(1);
		const [url, body] = post.mock.calls[0];
		expect(url).toBe('/sdk-config/rules');
		expect(body.name).toBe('Checkout journey');
		expect(body.conditions).toEqual({ all: [{ key: 'user.id', op: 'eq', value: 'u-42' }] });
		expect(body.settings.triggers).toHaveLength(1);
		expect(JSON.stringify(body.settings.triggers[0])).toContain('Payment declined');
	});

	it('keeps what was typed into an existing rule when switching to JSON', async () => {
		const rule: SdkConfigRule = {
			id: 7,
			name: 'Quiet logs in production',
			position: 0,
			enabled: true,
			conditions: { all: [{ key: 'deployment.environment.name', op: 'eq', value: 'production' }] },
			settings: { schemaVersion: 1, signals: { console: { mode: 'buffer', minSeverity: 'warn' } } },
			updatedAt: '2026-11-27T10:00:00Z'
		};
		render(RuleDialog, { open: true, rule, onSaved: vi.fn() });

		await fireEvent.input(nameInput(), { target: { value: 'Quiet logs everywhere' } });
		await fireEvent.click(tab(/^JSON/));
		expect(activeTab()?.textContent?.trim()).toBe('JSON');
		expect(nameInput().value).toBe('Quiet logs everywhere');

		await fireEvent.click(tab(/^Signals/));
		expect(activeTab()?.textContent?.trim()).toBe('Signals (1)');

		await fireEvent.click(screen.getByRole('button', { name: 'Update Rule' }));
		expect(put).toHaveBeenCalledTimes(1);
		const [url, body] = put.mock.calls[0];
		expect(url).toBe('/sdk-config/rules/7');
		expect(body.name).toBe('Quiet logs everywhere');
		expect(body.settings).toEqual(rule.settings);
	});
});

describe('log-line trigger builder', () => {
	it.each(['browser', 'flutter', 'ios', 'android'] as const)(
		'offers a minimum level on %s, whose logs all carry one',
		(platform) => {
			render(LogTriggerBuilder, { platform, onAdd: vi.fn() });
			expect(screen.getByText('any level')).toBeTruthy();
		}
	);
});
