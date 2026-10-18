'use strict';
const express = require('express');
const Redis = require('ioredis');
const { Pool } = require('pg');
const { catalogRouter } = require('./routes/catalog');
const { reviewsRouter } = require('./routes/reviews');

const db = new Pool({ connectionString: process.env.DATABASE_URL });
const redis = new Redis(process.env.REDIS_URL || 'redis://cache.internal:6379');

const app = express();
app.use(catalogRouter({ db, redis }));
app.use(reviewsRouter({ db, redis }));
app.listen(Number(process.env.PORT || 3000));
