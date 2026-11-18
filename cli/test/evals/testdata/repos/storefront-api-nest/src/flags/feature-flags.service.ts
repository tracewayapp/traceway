import { Injectable } from '@nestjs/common';
import { Pool } from 'pg';

export interface FeatureFlag {
  key: string;
  enabled: boolean;
  rolloutPercent: number;
  rules: unknown;
}

export type FlagSet = Map<string, FeatureFlag>;

@Injectable()
export class FeatureFlagsService {
  constructor(private readonly db: Pool) {}

  /** The active flags, read from the database. */
  async load(): Promise<FlagSet> {
    const { rows } = await this.db.query(
      'SELECT key, enabled, rollout_percent, rules FROM feature_flags WHERE archived_at IS NULL ORDER BY key',
    );
    return new Map(
      rows.map((r) => [r.key, { key: r.key, enabled: r.enabled, rolloutPercent: r.rollout_percent, rules: r.rules }]),
    );
  }
}
