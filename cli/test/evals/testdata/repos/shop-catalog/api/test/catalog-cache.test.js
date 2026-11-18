'use strict';
const test = require('node:test');
const assert = require('node:assert');
const { CACHE_TTL_SECONDS, cacheKey } = require('../src/catalog-cache');

test('catalog cache keys are namespaced', () => {
  assert.match(cacheKey('/api/catalog?locale=en-US'), /^catalog:/);
  assert.ok(CACHE_TTL_SECONDS >= 60);
});
