import { Injectable, NestMiddleware } from "@nestjs/common";
import { Request, Response, NextFunction } from "express";
import * as traceway from "@traceway/backend";

@Injectable()
export class TracewayMiddleware implements NestMiddleware {
  use(req: Request, res: Response, next: NextFunction) {
    const traceId = crypto.randomUUID().replace(/-/g, "");
    const startedAt = new Date();
    const startMs = performance.now();

    (req as any).traceId = traceId;

    res.on("finish", () => {
      const durationMs = performance.now() - startMs;
      const endpoint = `${req.method} ${req.route?.path || req.path}`;

      traceway.captureTrace(
        traceId,
        endpoint,
        durationMs,
        startedAt,
        res.statusCode,
        0,
        req.ip || ""
      );
    });

    next();
  }
}
