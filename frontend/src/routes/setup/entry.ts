import { api } from '$lib/api';

export interface SetupEntry {
	existingAccount: boolean;
	organizationId: number | null;
}

function parseOrganizationId(raw: string | null): number | null {
	if (!raw || !/^\d+$/.test(raw)) return null;
	const id = Number(raw);
	return Number.isSafeInteger(id) && id > 0 ? id : null;
}

/**
 * How /setup opens, or null when only a pending proposal could still keep an account with projects here.
 * A zero-project account always gets the onboarding flow; an account with projects gets it when the link asks for
 * it (`?mode=add`, `?organizationId=<id>`).
 */
export function setupEntry(params: URLSearchParams, cachedProjects: number): SetupEntry | null {
	const organizationId = parseOrganizationId(params.get('organizationId'));
	if (cachedProjects === 0) return { existingAccount: false, organizationId };
	if (params.get('mode') === 'add' || organizationId !== null) {
		return { existingAccount: true, organizationId };
	}
	return null;
}

export function setupOrganization(
	writable: { id: number }[],
	requested: number | null,
	current: number | null | undefined,
	picked: number | null
): number | null {
	const available = (id: number | null | undefined) => writable.some((org) => org.id === id);
	if (picked !== null && available(picked)) return picked;
	if (requested !== null) return available(requested) ? requested : null;
	return current != null && available(current) ? current : (writable[0]?.id ?? null);
}

/** The first of the organizations whose latest setup proposal still waits for a decision. */
export async function findPendingProposalOrganization(
	organizationIds: number[]
): Promise<number | null> {
	const statuses = await Promise.all(
		organizationIds.map(async (id) => {
			try {
				const response = (await api.get(`/setup/drafts?organizationId=${id}`)) as {
					draft?: { status?: string } | null;
				};
				return response?.draft?.status === 'pending';
			} catch {
				return false;
			}
		})
	);
	const index = statuses.indexOf(true);
	return index === -1 ? null : organizationIds[index];
}
