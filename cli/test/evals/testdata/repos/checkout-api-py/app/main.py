import logging
import time

from fastapi import FastAPI, Request

from app import cart, checkout

app = FastAPI(title="checkout-api")
app.include_router(cart.router)
app.include_router(checkout.router)

access = logging.getLogger("app.access")
access.setLevel(logging.INFO)


@app.middleware("http")
async def access_log(request: Request, call_next):
    """One line per request ("GET /api/cart 200 4ms"), inside the request's trace."""
    start = time.perf_counter()
    status = 500
    try:
        response = await call_next(request)
        status = response.status_code
        return response
    finally:
        ms = int((time.perf_counter() - start) * 1000)
        route = getattr(request.scope.get("route"), "path", "")
        access.info(
            "%s %s %d %dms", request.method, request.url.path, status, ms,
            extra={"http.request.method": request.method, "url.path": request.url.path, "http.route": route,
                   "http.response.status_code": status, "duration_ms": ms},
        )


SHIPPING_OPTIONS = [
    {"id": "standard", "label": "Standard (3-5 days)", "priceCents": 0},
    {"id": "express", "label": "Express (1-2 days)", "priceCents": 990},
]


@app.get("/api/shipping/options")
def shipping_options():
    return SHIPPING_OPTIONS
