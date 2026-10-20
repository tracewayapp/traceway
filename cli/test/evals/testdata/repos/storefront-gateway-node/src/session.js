'use strict';
const Redis = require('ioredis');

const redis = new Redis(process.env.REDIS_URL || 'redis://cache.internal:6379');

function session() {
  return async (req, res, next) => {
    const sid = req.get('cookie')?.match(/sid=([a-f0-9]+)/)?.[1];
    const raw = sid ? await redis.get(`sess:${sid}`) : null;
    if (!raw) {
      return res.status(401).json({ error: 'not signed in' });
    }
    req.session = JSON.parse(raw);
    next();
  };
}

module.exports = { session };
