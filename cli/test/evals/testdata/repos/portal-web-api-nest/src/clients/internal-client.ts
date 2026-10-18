import { Injectable } from '@nestjs/common';

const BASE = {
  accounts: process.env.ACCOUNTS_URL ?? 'http://accounts-service.internal:8080',
  billing: process.env.BILLING_URL ?? 'http://billing-service.internal:8080',
  notifications: process.env.NOTIFICATIONS_URL ?? 'http://notifications-service.internal:8080',
  recommendations: process.env.RECOMMENDATIONS_URL ?? 'http://recommendations-service.internal:8080',
};

export type Service = keyof typeof BASE;

@Injectable()
export class InternalClient {
  async get<T>(service: Service, path: string): Promise<T> {
    const res = await fetch(BASE[service] + path, { headers: { accept: 'application/json' } });
    if (!res.ok) {
      throw new Error(`${service} ${path} answered ${res.status}`);
    }
    return (await res.json()) as T;
  }
}
