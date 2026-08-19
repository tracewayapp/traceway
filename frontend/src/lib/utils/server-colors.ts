export const SERVER_COLORS = [
	'#e97a35', // Orange
	'#2a9d8f', // Teal
	'#4361ee', // Blue
	'#d664ba', // Pink
	'#e9c46a', // Yellow
	'#9b5de5', // Purple
	'#57cc99', // Green
	'#ef476f' // Red
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
