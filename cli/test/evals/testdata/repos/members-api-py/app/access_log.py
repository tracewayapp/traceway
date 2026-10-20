"""One log line per request ("GET /api/search 200 14ms"), inside the request's trace."""

import logging
import time

from fastapi import FastAPI, Request

log = logging.getLogger("app.access")


def install(app: FastAPI) -> None:
    @app.middleware("http")
    async def access_log(request: Request, call_next):
        start = time.perf_counter()
        status = 500
        try:
            response = await call_next(request)
            status = response.status_code
            return response
        finally:
            ms = round((time.perf_counter() - start) * 1000)
            route = request.scope.get("route")
            log.info(
                "%s %s %d %dms", request.method, request.url.path, status, ms,
                extra={
                    "http.request.method": request.method,
                    "url.path": request.url.path,
                    "http.route": getattr(route, "path", ""),
                    "http.response.status_code": status,
                    "duration_ms": ms,
                },
            )
