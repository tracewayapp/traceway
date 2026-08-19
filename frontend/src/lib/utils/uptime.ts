export function uptimeClass(uptime: number): string {
	if (uptime >= 99.9) return 'text-green-600 dark:text-green-400';
	if (uptime >= 99) return '';
	return 'text-red-600 dark:text-red-400';
}
