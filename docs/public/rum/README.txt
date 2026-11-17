RUM screenshots — captured 16 November 2026

Source: the current v2.1 Traceway dashboard, serving real browser telemetry from
testing/shop. The isolated demo deliberately slows release 0.2.0. Screenshots
show synthetic demo visits, not customer data or production benchmarks.

Reproduce the dataset and screenshots with the stack and command documented in:
testing/shop/traffic/client-metrics-e2e.mjs

Capture at 1440 x 1100 with Chromium, UTC timezone. The dashboard image is a
viewport screenshot of Web Performance; the test also saves a full-page copy.
The release image filters release 0.2.0 to Android. Other images show a linked
slow session, release-specific SLOs, and the breached slow-release SLO detail.
Dashboard and SLO detail use the demo window 21:52–22:02 UTC on 16 November.

The website's matching assets live in website/public/images/rum-*.png.
No data, chart values or UI labels have been retouched.

Diagrams (architecture.png, ring-buffer.png, metrics-pipeline.png) are exported
from diagrams/docs/rum/*.drawio; the website's dark versions
(website/public/images/rum-architecture.png, ...) from diagrams/website/rum/.
diagrams/render-drawio.mjs renders them without the draw.io desktop app.
