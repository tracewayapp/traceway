'use strict';
const express = require('express');
const { Pool } = require('pg');
const clients = require('./clients');
const { session } = require('./session');
const { cartRoute } = require('./routes/cart');
const { wishlistRoute } = require('./routes/wishlist');

const db = new Pool({ connectionString: process.env.DATABASE_URL });
const app = express();
app.use(session());
app.get('/api/cart', cartRoute(clients));
app.get('/api/wishlist', wishlistRoute({ db, catalog: clients.catalog }));

app.listen(Number(process.env.PORT || 3000));
