// Winter holiday decorations show from November 27 to January 7, so a
// self-hosted image keeps them out of the rest of the year without a release.
export function isHolidaySeason(now: Date): boolean {
	const month = now.getMonth();
	const day = now.getDate();
	return month === 11 || (month === 10 && day >= 27) || (month === 0 && day <= 7);
}

export const holidaySeason = isHolidaySeason(new Date());
