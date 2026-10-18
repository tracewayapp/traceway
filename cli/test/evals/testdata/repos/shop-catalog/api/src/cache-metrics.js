'use strict';
const { metrics } = require('@opentelemetry/api');

// Hit ratio per cache over each export interval, as the cache.hit_ratio
// gauge (attribute: cache).
const counts = new Map();

function record(cache, hit) {
  const c = counts.get(cache) ?? { hit: 0, miss: 0 };
  c[hit ? 'hit' : 'miss'] += 1;
  counts.set(cache, c);
}

metrics
  .getMeter('catalog-api')
  .createObservableGauge('cache.hit_ratio', { unit: '1', description: 'Share of cache lookups that hit' })
  .addCallback((result) => {
    for (const [cache, c] of counts) {
      const total = c.hit + c.miss;
      if (total > 0) {
        result.observe(c.hit / total, { cache });
      }
      counts.set(cache, { hit: 0, miss: 0 });
    }
  });

module.exports = { record };
