// Thanksgiving decorations only show in the second half of November, so a
// self-hosted image keeps them out of the rest of the year without a release.
export function isHarvestSeason(now: Date): boolean {
	return now.getMonth() === 10 && now.getDate() >= 15;
}

export const harvestSeason = isHarvestSeason(new Date());
