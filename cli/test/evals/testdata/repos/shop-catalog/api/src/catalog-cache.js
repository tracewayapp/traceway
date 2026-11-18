'use strict';

// How long a built catalog response stays in Redis.
const CACHE_TTL_SECONDS = 6 * 60 * 60;

// cacheKey is the Redis key a catalog request is cached under.
function cacheKey(originalUrl) {
  return `catalog:${originalUrl}`;
}

module.exports = { CACHE_TTL_SECONDS, cacheKey };
