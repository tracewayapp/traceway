import { api } from '$lib/api';
import { projectsState } from './projects.svelte';

class AgentState {
	needsInputCount = $state(0);
	pendingApprovalCount = $state(0);

	async refreshBadge() {
		try {
			const res = await api.get('/agent/attempts/badge', {
				projectId: projectsState.currentProjectId ?? undefined
			});
			this.needsInputCount = res.needsInput ?? 0;
			this.pendingApprovalCount = res.pendingApproval ?? 0;
		} catch {
			// badge stays at its last value
		}
	}
}

export const agentState = new AgentState();
