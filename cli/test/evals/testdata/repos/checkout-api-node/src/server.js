const { Pool } = require('pg');
const { createApp } = require('./app');
const { PgOrdersRepo } = require('./orders/pgRepo');
const { PaymentsClient } = require('./payments');
const { Mailer } = require('./mailer');

const pool = new Pool({ connectionString: process.env.DATABASE_URL });
const app = createApp({ repo: new PgOrdersRepo(pool), payments: new PaymentsClient(process.env.PAYMENTS_URL), mailer: new Mailer(pool) });
app.listen(Number(process.env.PORT ?? 8080));
