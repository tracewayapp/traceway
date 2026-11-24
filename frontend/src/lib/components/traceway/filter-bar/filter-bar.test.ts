import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { keyOperatorOptions, operatorFlags } from '$lib/utils/attribute-filters';
import FilterBar from './filter-bar.svelte';
import { attributeConfig } from './attributes';
import { traceIdField } from './presets';
import type { FilterField } from './fields';

const scrollIntoView = Object.getOwnPropertyDescriptor(HTMLElement.prototype, 'scrollIntoView');
beforeEach(() => {
	Object.defineProperty(HTMLElement.prototype, 'scrollIntoView', {
		configurable: true,
		value: vi.fn()
	});
});
afterEach(async () => {
	cleanup();
	// bits-ui releases the body scroll lock on a timer after a dialog closes.
	await new Promise((resolve) => setTimeout(resolve, 50));
	if (scrollIntoView)
		Object.defineProperty(HTMLElement.prototype, 'scrollIntoView', scrollIntoView);
	else Reflect.deleteProperty(HTMLElement.prototype, 'scrollIntoView');
});

const fields: FilterField[] = [
	{
		kind: 'select',
		key: 'kind',
		label: 'Kind',
		anyLabel: 'All kinds',
		options: [
			{ value: '2', label: 'Server' },
			{ value: '3', label: 'Client' }
		]
	},
	traceIdField('trace', 'Every span of one trace.'),
	{
		kind: 'select',
		key: 'status',
		label: 'Status',
		anyLabel: 'Active',
		emptyValue: 'active',
		narrowsByDefault: true,
		options: [{ value: 'resolved', label: 'Resolved' }]
	}
];
const route = { key: 'http.route', value: '/a', ...operatorFlags('=') };

function setup(overrides: Record<string, unknown> = {}) {
	const onApply = vi.fn();
	const result = render(FilterBar, {
		props: {
			search: 'checkout',
			searchPlaceholder: 'Search spans...',
			fields,
			values: { kind: '2', trace: '', status: 'active' },
			attributes: attributeConfig([route], { operators: keyOperatorOptions, requireValue: true }),
			onApply,
			noun: 'spans',
			rangeLabel: 'last 1h',
			...overrides
		}
	});
	return { onApply, ...result };
}

it('shows a pill per applied filter, a default pill, and keeps the search bar with Go', () => {
	setup();
	expect(screen.getByRole('button', { name: 'Edit filter Kind = Server' })).toBeTruthy();
	expect(screen.getByRole('button', { name: 'Edit filter http.route = /a' })).toBeTruthy();
	expect(screen.getByRole('button', { name: 'Edit filter Status = Active' })).toBeTruthy();
	expect(screen.queryByRole('button', { name: /Remove Status/ })).toBeNull();
	expect(screen.queryByRole('button', { name: /Trace ID/ })).toBeNull();
	expect(screen.getByRole('button', { name: 'Add filter' })).toBeTruthy();
	expect(screen.getByRole('button', { name: 'Go' })).toBeTruthy();
	expect(screen.getByRole('button', { name: 'Clear all' })).toBeTruthy();
});

it('runs the search from Go and Enter with the filters already applied', async () => {
	const { onApply } = setup();
	const box = screen.getByPlaceholderText('Search spans...');
	await fireEvent.input(box, { target: { value: ' payments ' } });
	await fireEvent.click(screen.getByRole('button', { name: 'Go' }));
	expect(onApply).toHaveBeenLastCalledWith({
		search: 'payments',
		values: { kind: '2', trace: '', status: 'active' },
		attributes: [route]
	});
	await fireEvent.keyDown(box, { key: 'Enter' });
	expect(onApply).toHaveBeenCalledTimes(2);
});

it('removes a pill at once, and clears all filters but keeps the search', async () => {
	const { onApply } = setup();
	await fireEvent.click(screen.getByRole('button', { name: 'Remove Kind filter' }));
	expect(onApply).toHaveBeenLastCalledWith({
		search: 'checkout',
		values: { kind: '', trace: '', status: 'active' },
		attributes: [route]
	});
	await fireEvent.click(screen.getByRole('button', { name: 'Clear all' }));
	expect(onApply).toHaveBeenLastCalledWith({
		search: 'checkout',
		values: { kind: '', trace: '', status: 'active' },
		attributes: []
	});
});

it('opens the drawer on the clicked field and applies only on Apply', async () => {
	const { onApply } = setup();
	await fireEvent.click(screen.getByRole('button', { name: 'Edit filter Kind = Server' }));
	const drawer = await screen.findByRole('dialog');
	expect(within(drawer).getByText('Applied to spans in the last 1h')).toBeTruthy();
	await waitFor(() => expect(document.activeElement?.closest('[id$="-kind"]')).not.toBeNull());
	await fireEvent.input(within(drawer).getByRole('textbox', { name: 'Trace ID' }), {
		target: { value: '4bf92f3577b34da6a3ce929d0e0e4736' }
	});
	expect(onApply).not.toHaveBeenCalled();
	await fireEvent.click(within(drawer).getByRole('button', { name: /^Apply/ }));
	expect(onApply).toHaveBeenLastCalledWith({
		search: 'checkout',
		values: { kind: '2', trace: '4bf92f3577b34da6a3ce929d0e0e4736', status: 'active' },
		attributes: [route]
	});
	await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
});

it('shows a bad trace ID once and keeps Apply from running', async () => {
	const { onApply } = setup();
	await fireEvent.click(screen.getByRole('button', { name: 'Add filter' }));
	const drawer = await screen.findByRole('dialog');
	await fireEvent.input(within(drawer).getByRole('textbox', { name: 'Trace ID' }), {
		target: { value: '123' }
	});
	expect(within(drawer).getAllByText('A trace ID is 32 hexadecimal characters.')).toHaveLength(1);
	await fireEvent.click(within(drawer).getByRole('button', { name: /^Apply/ }));
	expect(onApply).not.toHaveBeenCalled();
	expect(screen.getByRole('dialog')).toBeTruthy();
});

it('drops drawer edits when the drawer closes without Apply', async () => {
	const { onApply } = setup();
	await fireEvent.click(screen.getByRole('button', { name: 'Add filter' }));
	let drawer = await screen.findByRole('dialog');
	await fireEvent.input(within(drawer).getByRole('textbox', { name: 'Trace ID' }), {
		target: { value: 'abc' }
	});
	await fireEvent.keyDown(drawer, { key: 'Escape' });
	await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
	await fireEvent.click(screen.getByRole('button', { name: 'Add filter' }));
	drawer = await screen.findByRole('dialog');
	expect(
		(within(drawer).getByRole('textbox', { name: 'Trace ID' }) as HTMLInputElement).value
	).toBe('');
	expect(onApply).not.toHaveBeenCalled();
});

it('edits an attribute pill in the drawer, and opens a new one filled in from outside', async () => {
	const { onApply, component } = setup();
	await fireEvent.click(screen.getByRole('button', { name: 'Edit filter http.route = /a' }));
	let drawer = await screen.findByRole('dialog');
	await fireEvent.input(within(drawer).getByDisplayValue('/a'), { target: { value: '/b' } });
	await fireEvent.click(within(drawer).getByRole('button', { name: 'Update filter' }));
	await fireEvent.click(within(drawer).getByRole('button', { name: /^Apply/ }));
	expect(onApply.mock.lastCall?.[0].attributes).toEqual([{ ...route, value: '/b' }]);
	await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());

	(component as unknown as { openAttribute: (f: typeof route) => void }).openAttribute({
		key: 'user.id',
		value: 'u_1',
		...operatorFlags('=')
	});
	drawer = await screen.findByRole('dialog');
	expect(within(drawer).getByDisplayValue('user.id')).toBeTruthy();
	expect(within(drawer).getByDisplayValue('u_1')).toBeTruthy();
});

it('asks fields with loaded options again each time the drawer opens', async () => {
	const loadOptions = vi.fn().mockResolvedValue([{ value: 'web-1', label: 'web-1' }]);
	setup({
		fields: [{ kind: 'select', key: 'server', label: 'Instance', options: [], loadOptions }],
		values: { server: '' },
		attributes: undefined
	});
	await fireEvent.click(screen.getByRole('button', { name: 'Add filter' }));
	await screen.findByRole('dialog');
	expect(loadOptions).toHaveBeenCalledTimes(1);
	await fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' });
	await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
	await fireEvent.click(screen.getByRole('button', { name: 'Add filter' }));
	await screen.findByRole('dialog');
	expect(loadOptions).toHaveBeenCalledTimes(2);
});

it('shows a quick dropdown in the search bar and the drawer, and a pill only on phones', async () => {
	const { onApply } = setup({
		fields: [
			{
				kind: 'select',
				key: 'level',
				label: 'Level',
				emptyValue: '0',
				inSearch: true,
				options: [
					{ value: '13', label: 'WARN+' },
					{ value: '17', label: 'ERROR+' }
				]
			},
			fields[0]
		],
		values: { level: '13', kind: '' },
		attributes: undefined
	});
	const dropdown = screen.getByRole('button', { name: 'Level' });
	expect(dropdown.textContent).toContain('WARN+');
	// Desktop shows the dropdown; phones hide it and show the value as a pill instead.
	expect(dropdown.className).toMatch(/(^|\s)hidden(\s|$)/);
	expect(dropdown.className).toContain('sm:flex');
	const phonePill = screen.getByRole('button', { name: 'Edit filter Level = WARN+' });
	expect(phonePill.parentElement?.className).toContain('sm:hidden');
	expect(screen.queryByRole('button', { name: 'Clear all' })).toBeNull();

	// The dropdown waits for Go, like the search text.
	await fireEvent.keyDown(screen.getByRole('button', { name: 'Level' }), { key: 'ArrowDown' });
	await fireEvent.pointerUp(await screen.findByRole('option', { name: 'ERROR+' }), {
		pointerType: 'mouse'
	});
	expect(onApply).not.toHaveBeenCalled();
	await fireEvent.click(screen.getByRole('button', { name: 'Go' }));
	expect(onApply.mock.lastCall?.[0].values).toEqual({ level: '17', kind: '' });

	// The drawer lists it too, with the value the bar shows.
	await fireEvent.click(screen.getByRole('button', { name: 'Add filter' }));
	const drawer = await screen.findByRole('dialog');
	expect(within(drawer).getByRole('button', { name: /^Level/ }).textContent).toContain('ERROR+');
	expect(within(drawer).getByRole('button', { name: 'Kind' })).toBeTruthy();
});
