export const SERVER_COLORS = [
	'var(--chart-1)',
	'var(--chart-2)',
	'var(--chart-3)',
	'var(--chart-4)',
	'var(--chart-5)',
	'oklch(0.7 0.15 300)', // Purple
	'oklch(0.7 0.15 120)', // Green
	'oklch(0.65 0.2 0)' // Red
] as const;

export function getServerColor(serverName: string, allServers: string[]): string {
	const sortedServers = [...allServers].sort();
	const index = sortedServers.indexOf(serverName);
	return SERVER_COLORS[index % SERVER_COLORS.length];
}

export function getServerColorMap(servers: string[]): Record<string, string> {
	const map: Record<string, string> = {};
	const sortedServers = [...servers].sort();
	sortedServers.forEach((server, index) => {
		map[server] = SERVER_COLORS[index % SERVER_COLORS.length];
	});
	return map;
}
