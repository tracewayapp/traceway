import { Injectable } from '@nestjs/common';
import { InternalClient } from '../clients/internal-client';
import { DashboardPresenter, Profile, BillingSummary, Unread, Suggestion } from './dashboard.presenter';

@Injectable()
export class DashboardService {
  constructor(
    private readonly client: InternalClient,
    private readonly presenter: DashboardPresenter,
  ) {}

  // Everything the portal home page shows for one account.
  async load(accountId: string) {
    const id = encodeURIComponent(accountId);
    const profile = await this.client.get<Profile>('accounts', `/internal/accounts/${id}/profile`);
    const billing = await this.client.get<BillingSummary>('billing', `/internal/billing/${id}/summary`);
    const unread = await this.client.get<Unread>('notifications', `/internal/notifications/unread?accountId=${id}`);
    const suggestions = await this.client.get<Suggestion[]>(
      'recommendations',
      `/internal/recommendations?accountId=${id}&limit=8`,
    );
    return this.presenter.compose(profile, billing, unread, suggestions);
  }
}
