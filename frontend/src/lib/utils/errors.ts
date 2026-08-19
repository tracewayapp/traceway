export function getErrorStatus(error: unknown): number | undefined {
	if (!error || typeof error !== 'object' || !('status' in error)) return undefined;
	return typeof error.status === 'number' ? error.status : undefined;
}

export function getErrorMessage(error: unknown, fallback = ''): string {
	if (error instanceof Error) return error.message || fallback;
	if (!error || typeof error !== 'object' || !('message' in error)) return fallback;
	return typeof error.message === 'string' ? error.message || fallback : fallback;
}
