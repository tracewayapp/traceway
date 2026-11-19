'use strict';
const winston = require('winston');

const logger = winston.createLogger({ level: 'info', transports: [new winston.transports.Console()] });

// One line per request ("GET /api/cart 200 331ms"), inside the request's trace.
function accessLog(req, res, next) {
  const start = process.hrtime.bigint();
  res.on('finish', () => {
    const ms = Number((process.hrtime.bigint() - start) / 1000000n);
    logger.info(`${req.method} ${req.path} ${res.statusCode} ${ms}ms`, {
      'http.request.method': req.method,
      'url.path': req.path,
      'http.route': req.route?.path ?? '',
      'http.response.status_code': res.statusCode,
      duration_ms: ms,
    });
  });
  next();
}

module.exports = { accessLog };
