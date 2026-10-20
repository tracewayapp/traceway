const winston = require('winston');

const logger = winston.createLogger({ level: 'info', transports: [new winston.transports.Console()] });

// One line per request ("GET /api/orders 200 7ms"), inside the request's trace.
function accessLog(req, res, next) {
  const start = process.hrtime.bigint();
  res.on('finish', () => {
    const ms = Number((process.hrtime.bigint() - start) / 1000000n);
    const path = req.originalUrl.split('?')[0];
    logger.info(`${req.method} ${path} ${res.statusCode} ${ms}ms`, {
      'http.request.method': req.method,
      'url.path': path,
      'http.route': req.route ? req.baseUrl + req.route.path : '',
      'http.response.status_code': res.statusCode,
      duration_ms: ms,
    });
  });
  next();
}

module.exports = { accessLog };
