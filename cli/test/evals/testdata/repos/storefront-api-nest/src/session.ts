import type { NextFunction, Request, Response } from 'express';

// The storefront edge validates the session cookie and forwards the signed-in
// customer as x-customer-id; anonymous requests arrive without it.
export function session(req: Request & { customerId?: number }, _res: Response, next: NextFunction) {
  const id = Number(req.get('x-customer-id'));
  if (Number.isInteger(id) && id > 0) {
    req.customerId = id;
  }
  next();
}
