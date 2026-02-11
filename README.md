<p align="center">
  <img src="Traceway Logo.png" alt="Traceway Logo" width="400" />
</p>

<h3 align="center">Error tracking and performance monitoring for Go and Node.js</h3>

<p align="center">
  <a href="https://tracewayapp.com">Website</a> · <a href="https://docs.tracewayapp.com">Docs</a> · <a href="https://github.com/tracewayapp/go-client">Go Client SDK</a>
</p>

---

<img width="2452" height="1966" alt="Traceway Dashboard" src="https://github.com/user-attachments/assets/30a4fa24-7d08-4b36-a8f3-42abc73692fd" />

## Features

- **Issue Tracking** — Automatic exception grouping with stack traces and contextual tags
- **Endpoint Performance** — P50, P95, P99 latency percentiles with Apdex scoring
- **System Metrics** — CPU, memory, goroutines, and GC monitoring
- **Distributed Tracing** — Full request traces with span breakdowns
- **Background Tasks** — Track and monitor async job performance

## Quick Start

```bash
docker compose up --build
```

See the [docs](https://docs.tracewayapp.com) for all deployment options and configuration.

## Documentation

Full documentation is available at **[docs.tracewayapp.com](https://docs.tracewayapp.com)**:

- **SDK Integration** — Set up the Go or Node.js client in your application
- **Self-Hosting** — Deploy with Docker Compose or on your own infrastructure
- **Concepts** — How tracing, exception grouping, and metrics work

## More Screenshots

See the [`/printscreens`](./printscreens) directory for screenshots of all pages.

## Project Structure

| Directory | Description |
|-----------|-------------|
| `backend/` | Go/Gin API server with ClickHouse and PostgreSQL |
| `frontend/` | SvelteKit 2 dashboard application |
| `website/` | Landing page |
| `docs/` | Documentation source |

## Links

- [Website](https://tracewayapp.com)
- [Documentation](https://docs.tracewayapp.com)
- [Go Client SDK](https://github.com/tracewayapp/go-client)
