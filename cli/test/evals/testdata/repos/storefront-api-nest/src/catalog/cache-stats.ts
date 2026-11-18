import { Injectable } from '@nestjs/common';
import { metrics } from '@opentelemetry/api';

// Counts Redis lookups for product data and reports the share answered from
// the cache over each export interval.
@Injectable()
export class CacheStats {
  private hits = 0;
  private misses = 0;

  constructor() {
    metrics
      .getMeter('storefront-api')
      .createObservableGauge('cache.hit_ratio', { unit: '1', description: 'Product cache lookups answered by Redis' })
      .addCallback((result) => {
        const total = this.hits + this.misses;
        if (total > 0) {
          result.observe(this.hits / total, { 'cache.name': 'product' });
        }
        this.hits = 0;
        this.misses = 0;
      });
  }

  hit() {
    this.hits++;
  }

  miss() {
    this.misses++;
  }
}
