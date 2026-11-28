import { describe, expect, it } from 'vitest';
import { isHolidaySeason } from './holiday';

describe('isHolidaySeason', () => {
	it('covers November 27 to January 7', () => {
		expect(isHolidaySeason(new Date(2026, 10, 27))).toBe(true);
		expect(isHolidaySeason(new Date(2026, 11, 1))).toBe(true);
		expect(isHolidaySeason(new Date(2026, 11, 25))).toBe(true);
		expect(isHolidaySeason(new Date(2026, 11, 31, 23, 59))).toBe(true);
		expect(isHolidaySeason(new Date(2027, 0, 1))).toBe(true);
		expect(isHolidaySeason(new Date(2027, 0, 7, 23, 59))).toBe(true);
	});

	it('is off the rest of the year', () => {
		expect(isHolidaySeason(new Date(2026, 10, 26, 23, 59))).toBe(false);
		expect(isHolidaySeason(new Date(2027, 0, 8))).toBe(false);
		expect(isHolidaySeason(new Date(2026, 6, 20))).toBe(false);
	});
});
