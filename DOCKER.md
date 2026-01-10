# Traceway Docker Deployment

All-in-one Docker container that runs the complete Traceway stack:
- **Frontend**: SvelteKit SPA served via nginx (port 80)
- **Backend**: Go/Gin API server (port 8082)
- **ClickHouse**: Time-series database (internal)

## Build

```bash
docker build -t traceway:latest .
```

## Run

### Basic
```bash
docker run -d --name traceway -p 80:80 -p 8082:8082 traceway:latest
```

### With Persistent Data (Recommended)
```bash
docker run -d --name traceway \
    -p 80:80 \
    -p 8082:8082 \
    -v traceway-data:/var/lib/clickhouse \
    traceway:latest
```

### With Custom Tokens
```bash
docker run -d --name traceway \
    -p 80:80 \
    -p 8082:8082 \
    -e TOKEN="your-client-token" \
    -e APP_TOKEN="your-app-token" \
    -v traceway-data:/var/lib/clickhouse \
    traceway:latest
```

## Access Points

| URL | Description |
|-----|-------------|
| `http://localhost/` | Frontend (SvelteKit SPA) |
| `http://localhost/api/*` | Backend API (proxied through nginx) |
| `http://localhost:8082/api/*` | Direct backend access |

## Useful Commands

```bash
# View logs
docker logs traceway

# Follow logs
docker logs -f traceway

# Check service status
docker exec traceway supervisorctl status

# Enter container shell
docker exec -it traceway bash

# Stop container
docker stop traceway

# Stop and remove
docker stop traceway && docker rm traceway

# Remove with data volume
docker stop traceway && docker rm traceway && docker volume rm traceway-data
```

## Architecture

The container uses **supervisord** to manage three services:

1. **ClickHouse** (priority 10) - Starts first
2. **Backend** (priority 20) - Waits for ClickHouse, then starts
3. **Nginx** (priority 30) - Starts last, serves frontend and proxies API

## Notes

- First startup takes ~60 seconds while ClickHouse initializes
- Backend automatically runs database migrations on startup
- Use `-v traceway-data:/var/lib/clickhouse` to persist data between restarts
- Environment variables `TOKEN` and `APP_TOKEN` can be overridden at runtime
