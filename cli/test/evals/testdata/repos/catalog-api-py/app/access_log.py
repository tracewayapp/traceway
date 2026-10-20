"""One log line per request ("GET /api/products/42 200 3ms"), inside the request's trace."""

import logging
import time

log = logging.getLogger("app.access")
log.setLevel(logging.INFO)


class AccessLog:
    def __init__(self, app):
        self.app = app

    async def __call__(self, scope, receive, send):
        if scope["type"] != "http":
            return await self.app(scope, receive, send)
        start = time.perf_counter()
        status = 500

        async def send_status(message):
            nonlocal status
            if message["type"] == "http.response.start":
                status = message["status"]
            await send(message)

        try:
            await self.app(scope, receive, send_status)
        finally:
            ms = int((time.perf_counter() - start) * 1000)
            route = getattr(scope.get("route"), "path", "")
            log.info(
                "%s %s %d %dms", scope["method"], scope["path"], status, ms,
                extra={"http.request.method": scope["method"], "url.path": scope["path"], "http.route": route,
                       "http.response.status_code": status, "duration_ms": ms},
            )
