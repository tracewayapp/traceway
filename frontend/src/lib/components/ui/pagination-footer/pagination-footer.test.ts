import { afterEach, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';
import PaginationFooter from './pagination-footer.svelte';

afterEach(() => cleanup());

function setup(loading: boolean) {
	const onPageChange = vi.fn();
	const onPageSizeChange = vi.fn();
	const result = render(PaginationFooter, {
		props: {
			currentPage: 2,
			totalPages: 5,
			pageSize: 20,
			totalItems: 100,
			onPageChange,
			onPageSizeChange,
			loading
		}
	});
	return { onPageChange, onPageSizeChange, ...result };
}

const button = (name: string) => screen.getByRole('button', { name }) as HTMLButtonElement;

it('locks paging and the page size while the list is loading', async () => {
	const { onPageChange, rerender } = setup(true);
	for (const name of ['Go to previous page', 'Go to next page']) {
		expect(button(name).disabled, name).toBe(true);
	}
	expect(screen.getByText('20').closest('button')?.disabled).toBe(true);
	await fireEvent.click(button('Go to next page'));
	expect(onPageChange).not.toHaveBeenCalled();

	await rerender({ loading: false });
	expect(screen.getByText('20').closest('button')?.disabled).toBe(false);
	await fireEvent.click(button('Go to next page'));
	expect(onPageChange).toHaveBeenCalledWith(3);
});
