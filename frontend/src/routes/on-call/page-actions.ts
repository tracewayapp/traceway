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
		toast.success(`Page ${done}`, { position: 'top-center' });
	} catch (e: unknown) {
		if ((e as { status?: number }).status === 409) {
			toast.error(`Page is already ${await conflictStatus(id)}`, { position: 'top-center' });
		} else {
			toast.error(`Failed to ${action} page`, { position: 'top-center' });
		}
	}
	oncallState.refreshOpenCount();
}
