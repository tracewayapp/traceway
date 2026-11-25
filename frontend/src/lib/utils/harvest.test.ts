import { describe, expect, it } from 'vitest';
import { isHarvestSeason } from './harvest';

describe('isHarvestSeason', () => {
	it('covers the second half of November', () => {
		expect(isHarvestSeason(new Date(2026, 10, 15))).toBe(true);
		expect(isHarvestSeason(new Date(2026, 10, 26))).toBe(true);
		expect(isHarvestSeason(new Date(2026, 10, 30, 23, 59))).toBe(true);
	});

	it('is off the rest of the year', () => {
		expect(isHarvestSeason(new Date(2026, 10, 14, 23, 59))).toBe(false);
		expect(isHarvestSeason(new Date(2026, 11, 1))).toBe(false);
		expect(isHarvestSeason(new Date(2026, 6, 20))).toBe(false);
	});
});
