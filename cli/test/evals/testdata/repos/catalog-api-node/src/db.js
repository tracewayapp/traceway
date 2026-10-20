'use strict';

const { Pool } = require('pg');

const pool = new Pool({
  connectionString: process.env.DATABASE_URL,
  max: 20,
});

module.exports = {
  query: (text, params) => pool.query(text, params),
};
