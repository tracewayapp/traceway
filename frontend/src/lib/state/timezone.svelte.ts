import { DateTime } from 'luxon';
import { projectsState } from './projects.svelte';
import { authState } from './auth.svelte';

export function getTimezone(): string {
	const organizationId = projectsState.currentProject?.organizationId;
	if (organizationId) {
		const timezone = authState.getTimezoneForOrganization(organizationId);
		if (timezone) return timezone;
	}
	// Before the project list resolves there is no current project to pick the
	// organization from. Fall back to the sole organization's timezone so the
	// resolved zone doesn't flip mid-session once projects load — wall-clock
	// state materialized under one zone must not be reinterpreted under another.
	if (authState.organizations.length === 1 && authState.organizations[0].timezone) {
		return authState.organizations[0].timezone;
	}
	return DateTime.local().zoneName || 'UTC';
}
