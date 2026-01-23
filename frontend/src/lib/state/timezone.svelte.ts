import { DateTime } from 'luxon';
import { projectsState } from './projects.svelte';
import { authState } from './auth.svelte';

export function getTimezone(): string {
	const organizationId = projectsState.currentProject?.organizationId;
	if (organizationId) {
		const timezone = authState.getTimezoneForOrganization(organizationId);
		if (timezone) return timezone;
	}
	return DateTime.local().zoneName || 'UTC';
}
