import { describe, it, expect } from 'vitest';
import { mergeFlameTrees } from './flame-diff';

describe('mergeFlameTrees', () => {
	it('returns null when there is no current tree', () => {
		expect(mergeFlameTrees({ name: 'root', value: 10 }, null)).toBeNull();
	});

	it('computes delta as the difference of normalized fractions', () => {
		const base = {
			name: 'root',
			value: 100,
			children: [
				{ name: 'a', value: 50 },
				{ name: 'b', value: 50 }
			]
		};
		const cur = {
			name: 'root',
			value: 200,
			children: [
				{ name: 'a', value: 150 },
				{ name: 'b', value: 50 }
			]
		};
		const merged = mergeFlameTrees(base, cur)!;
		expect(merged.value).toBe(200);
		const a = merged.children!.find((c) => c.name === 'a')!;
		const b = merged.children!.find((c) => c.name === 'b')!;
		// a: 150/200 - 50/100 = 0.75 - 0.5 = 0.25 (hotter)
		expect(a.delta).toBeCloseTo(0.25, 5);
		// b: 50/200 - 50/100 = 0.25 - 0.5 = -0.25 (cooler)
		expect(b.delta).toBeCloseTo(-0.25, 5);
	});

	it('treats frames absent from the baseline as fully added (positive delta)', () => {
		const base = { name: 'root', value: 100, children: [{ name: 'a', value: 100 }] };
		const cur = {
			name: 'root',
			value: 100,
			children: [
				{ name: 'a', value: 60 },
				{ name: 'new', value: 40 }
			]
		};
		const merged = mergeFlameTrees(base, cur)!;
		const added = merged.children!.find((c) => c.name === 'new')!;
		expect(added.delta).toBeCloseTo(0.4, 5);
	});
});
