const express = require('express');
const logger = require('./logger');
const telemetry = require('./telemetry');
const { accessLog } = require('./access-log');
const { createOrderHandler, getOrderHandler } = require('./orders/routes');

function createApp(deps) {
  const app = express();
  app.use(accessLog);
  app.use(express.json());
  app.post('/api/orders', createOrderHandler(deps));
  app.get('/api/orders/:orderId', getOrderHandler(deps));
  app.get('/api/carts/:cartId/order', getOrderHandler(deps));
  app.use((err, req, res, next) => {
    telemetry.recordException(err);
    logger.error(`request failed: ${err.message}`);
    res.status(500).json({ error: 'Something went wrong. Please try again.' });
  });
  return app;
}

module.exports = { createApp };
