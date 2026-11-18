import { Injectable } from '@nestjs/common';
import { trace } from '@opentelemetry/api';

export interface Profile { id: string; name: string; plan: string; seats: number; members: { userId: string; role: string }[] }
export interface BillingSummary { balanceCents: number; invoices: { number: string; status: string; dueAt: string }[] }
export interface Unread { count: number; latest: { id: string; kind: string; title: string }[] }
export interface Suggestion { itemId: string; category: string; score: number }

const tracer = trace.getTracer('web-api');

@Injectable()
export class DashboardPresenter {
  compose(profile: Profile, billing: BillingSummary, unread: Unread, suggestions: Suggestion[]) {
    return tracer.startActiveSpan('DashboardPresenter.compose', (span) => {
      try {
        return {
          account: { id: profile.id, name: profile.name, plan: profile.plan, seats: profile.seats },
          team: profile.members.length,
          balance: billing.balanceCents,
          invoices: billing.invoices.filter((i) => i.status !== 'paid'),
          notifications: unread,
          suggestions: suggestions.slice(0, 8),
        };
      } finally {
        span.end();
      }
    });
  }
}
