import type { NextFunction, Request, Response } from 'express';
import { createLogger, transports } from 'winston';

const logger = createLogger({ level: 'info', transports: [new transports.Console()] });

// One line per request ("GET /api/me 200 23ms"), inside the request's trace.
export function accessLog(req: Request, res: Response, next: NextFunction) {
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
