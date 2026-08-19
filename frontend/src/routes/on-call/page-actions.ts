import { toast } from 'svelte-sonner';
import { api } from '$lib/api';
import { projectsState } from '$lib/state/projects.svelte';
import { oncallState } from '$lib/state/oncall.svelte';

async function conflictStatus(id: number): Promise<string> {
	try {
		const res = await api.get(`/pages/${id}`, {
			projectId: projectsState.currentProjectId ?? undefined
		});
		return res?.page?.status ?? 'unknown';
	} catch {
		return 'unknown';
	}
}

// Runs an acknowledge/resolve call with the shared toast handling and sidebar
// badge refresh; callers refresh their own views afterwards.
export async function runPageAction(id: number, action: 'acknowledge' | 'resolve'): Promise<void> {
	const done = action === 'acknowledge' ? 'acknowledged' : 'resolved';
	try {
		await api.post(
			`/pages/${id}/${action}`,
			{},
			{ projectId: projectsState.currentProjectId ?? undefined }
		);
		toast.success(`Page ${done}`);
	} catch (e: unknown) {
		if ((e as { status?: number }).status === 409) {
			toast.error(`Page is already ${await conflictStatus(id)}`);
		} else {
			toast.error(`Failed to ${action} page`);
		}
	}
	oncallState.refreshOpenCount();
}

function pluralPages(n: number): string {
	return n === 1 ? 'page' : 'pages';
}

// Runs a bulk acknowledge/resolve with shared toasts and badge refresh.
// Returns true when the call succeeded (even if every page was skipped), so
// dialogs know whether to close.
export async function runBulkPageAction(
	ids: number[],
	action: 'acknowledge' | 'resolve',
	options: { archiveIssues?: boolean } = {}
): Promise<boolean> {
	const done = action === 'acknowledge' ? 'acknowledged' : 'resolved';
	try {
		const res = await api.post(
			`/pages/bulk-${action}`,
			action === 'resolve' ? { ids, archiveIssues: options.archiveIssues ?? false } : { ids },
			{ projectId: projectsState.currentProjectId ?? undefined }
		);
		const count = (action === 'acknowledge' ? res.acknowledged : res.resolved) ?? 0;
		const archived = res.archivedIssues ?? 0;
		if (count === 0 && archived === 0) {
			toast.error(ids.length === 1 ? `Page was already ${done}` : `Pages were already ${done}`);
		} else {
			let message =
				count === 1 && ids.length === 1 ? `Page ${done}` : `${count} ${pluralPages(count)} ${done}`;
			if (archived > 0) {
				message += ` and ${archived} issue${archived === 1 ? '' : 's'} archived`;
			}
			toast.success(message);
		}
		oncallState.refreshOpenCount();
		return true;
	} catch {
		toast.error(`Failed to ${action} ${pluralPages(ids.length)}`);
		oncallState.refreshOpenCount();
		return false;
	}
}
