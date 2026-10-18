import { DashboardService } from '../src/dashboard/dashboard.service';
import { DashboardPresenter } from '../src/dashboard/dashboard.presenter';

describe('DashboardService', () => {
  it('composes the dashboard from the four services', async () => {
    const responses: Record<string, unknown> = {
      accounts: { id: '42', name: 'Acme', plan: 'team', seats: 10, members: [{ userId: '1', role: 'owner' }] },
      billing: { balanceCents: 1200, invoices: [{ number: 'INV-1', status: 'open', dueAt: '2026-11-01' }] },
      notifications: { count: 2, latest: [] },
      recommendations: [{ itemId: 'a', category: 'x', score: 0.9 }],
    };
    const client = { get: jest.fn(async (service: string) => responses[service]) };
    const svc = new DashboardService(client as never, new DashboardPresenter());
    const out = await svc.load('42');
    expect(out.account.name).toBe('Acme');
    expect(out.invoices).toHaveLength(1);
    expect(client.get).toHaveBeenCalledTimes(4);
  });
});
