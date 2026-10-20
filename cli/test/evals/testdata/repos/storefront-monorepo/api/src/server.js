const express = require('express');
const { Pool } = require('pg');
const { ordersRouter } = require('./orders/routes');
const { authenticate } = require('./auth');

const pool = new Pool({ connectionString: process.env.DATABASE_URL });
const app = express();
app.use(express.json());
app.use('/api', authenticate(pool), ordersRouter(express, pool));
app.listen(Number(process.env.PORT ?? 8080));
