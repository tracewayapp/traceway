import { api } from '$lib/api';

export type TokenScope = 'traceway:read' | 'traceway:write';

export interface PersonalAccessToken {
	id: string;
	prefix: string;
	name: string;
	scope: TokenScope;
	lastUsedAt: string | null;
	expiresAt: string | null;
	createdAt: string;
}

export interface CreatedToken {
	id: string;
	name: string;
	prefix: string;
	token: string;
	scope: TokenScope;
	createdAt: string;
	expiresAt: string | null;
}

class PersonalAccessTokensState {
	tokens = $state<PersonalAccessToken[]>([]);
	loading = $state(false);
	error = $state('');

	async load() {
		this.loading = true;
		this.error = '';
		try {
			this.tokens = (await api.get('/personal-access-tokens')) as PersonalAccessToken[];
		} catch (e: unknown) {
			this.error = e instanceof Error ? e.message : 'Failed to load tokens';
		} finally {
			this.loading = false;
		}
	}

	async create(
		name: string,
		expiresInDays: number | null,
		scope: TokenScope
	): Promise<CreatedToken> {
		const res = (await api.post('/personal-access-tokens', {
			name,
			expiresInDays,
			scope
		})) as CreatedToken;
		await this.load();
		return res;
	}

	async revoke(id: string) {
		await api.delete(`/personal-access-tokens/${id}`);
		await this.load();
	}
}

export const personalAccessTokensState = new PersonalAccessTokensState();
