export type TimeRangePreset = { value: string; label: string; minutes: number };

// The time range picker's presets, in the groups it shows them.
export const presetGroups: readonly (readonly TimeRangePreset[])[] = [
	[
		{ value: '5m', label: '5 minutes', minutes: 5 },
		{ value: '30m', label: '30 minutes', minutes: 30 },
		{ value: '60m', label: '60 minutes', minutes: 60 }
	],
	[
		{ value: '3h', label: '3 hours', minutes: 180 },
		{ value: '6h', label: '6 hours', minutes: 360 },
		{ value: '12h', label: '12 hours', minutes: 720 },
		{ value: '24h', label: '24 hours', minutes: 1440 }
	],
	[
		{ value: '3d', label: '3 days', minutes: 4320 },
		{ value: '7d', label: '7 days', minutes: 10080 }
	],
	[
		{ value: '1M', label: '1 month', minutes: 43200 },
		{ value: '3M', label: '3 months', minutes: 129600 }
	]
];

export const presetMinutes: Record<string, number> = Object.fromEntries(
	presetGroups.flat().map((p) => [p.value, p.minutes])
);
