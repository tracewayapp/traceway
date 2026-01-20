# Traceway Docker Deployment

This document covers two deployment options:
1. **Minimal Image** (Recommended) - Small image (~20-30MB) with external ClickHouse
2. **All-in-One Image** - Larger image with systemd, suitable for development

---

## Minimal Deployment (Recommended)

Lightweight Docker image containing only the Go backend with embedded frontend. Designed for production deployments connecting to an external ClickHouse database.

### Features
- **Image size**: ~20-30MB (Alpine-based)
- **Single binary**: Go backend with embedded SvelteKit frontend
- **External ClickHouse**: Connect to any ClickHouse instance
- **Fast startup**: No init system overhead

### Build

```bash
docker build -f Dockerfile.minimal -t traceway:minimal .
```

### Required Environment Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `APP_TOKEN` | Dashboard authentication token | `your-secret-token` |
| `CLICKHOUSE_SERVER` | ClickHouse host:port | `clickhouse.example.com:9000` |
| `CLICKHOUSE_DATABASE` | Database name | `traceway` |
| `CLICKHOUSE_USERNAME` | DB username | `default` |
| `CLICKHOUSE_PASSWORD` | DB password | `password` |
| `CLICKHOUSE_TLS` | Enable TLS connection | `true` or `false` |

### Run

#### Basic
```bash
docker run -d --name traceway \
  -p 80:80 \
  -e APP_TOKEN="your-app-token" \
  -e CLICKHOUSE_SERVER="your-clickhouse-host:9000" \
  -e CLICKHOUSE_DATABASE="traceway" \
  -e CLICKHOUSE_USERNAME="default" \
  -e CLICKHOUSE_PASSWORD="your-password" \
  traceway:minimal
```

#### With TLS
```bash
docker run -d --name traceway \
  -p 80:80 \
  -e APP_TOKEN="your-app-token" \
  -e CLICKHOUSE_SERVER="your-clickhouse-host:9440" \
  -e CLICKHOUSE_DATABASE="traceway" \
  -e CLICKHOUSE_USERNAME="default" \
  -e CLICKHOUSE_PASSWORD="your-password" \
  -e CLICKHOUSE_TLS="true" \
  traceway:minimal
```

### Docker Compose Example

```yaml
version: '3.8'

services:
  traceway:
    build:
      context: .
      dockerfile: Dockerfile.minimal
    ports:
      - "80:80"
    environment:
      APP_TOKEN: "your-app-token"
      CLICKHOUSE_SERVER: "clickhouse:9000"
      CLICKHOUSE_DATABASE: "traceway"
      CLICKHOUSE_USERNAME: "default"
      CLICKHOUSE_PASSWORD: "your-password"
    depends_on:
      - clickhouse

  clickhouse:
    image: clickhouse/clickhouse-server:24.3
    ports:
      - "9000:9000"
      - "8123:8123"
    volumes:
      - clickhouse-data:/var/lib/clickhouse

volumes:
  clickhouse-data:
```

### Verify Image Size

```bash
docker images traceway:minimal
# Expected: ~20-30MB
```

### Access Points

| URL | Description |
|-----|-------------|
| `http://localhost/` | Frontend (SvelteKit SPA) |
| `http://localhost/api/*` | Backend API |
| `http://localhost/health` | Health check endpoint |

---

## All-in-One Deployment (with systemd)

Larger Docker image using systemd for process management. Suitable for development or when you want built-in process supervision.

### Build

```bash
docker build -t traceway:latest .
```

### Run

#### Basic
```bash
docker run -d --name traceway -p 80:80 -p 8082:8082 traceway:latest
```

#### With Persistent Data (Recommended)
```bash
docker run -d --name traceway \
    -p 80:80 \
    -p 8082:8082 \
    -v traceway-data:/var/lib/clickhouse \
    traceway:latest
```

#### With Custom Tokens
```bash
docker run -d --name traceway \
    -p 80:80 \
    -p 8082:8082 \
    -e TOKEN="your-client-token" \
    -e APP_TOKEN="your-app-token" \
    -v traceway-data:/var/lib/clickhouse \
    traceway:latest
```

### Access Points

| URL | Description |
|-----|-------------|
| `http://localhost/` | Frontend (SvelteKit SPA) |
| `http://localhost/api/*` | Backend API (proxied through nginx) |
| `http://localhost:8082/api/*` | Direct backend access |

---

## Useful Commands

```bash
# View logs
docker logs traceway

# Follow logs
docker logs -f traceway

# Enter container shell
docker exec -it traceway sh      # minimal image (alpine)
docker exec -it traceway bash    # all-in-one image (debian)

# Check health
curl http://localhost/health

# Stop container
docker stop traceway

# Stop and remove
docker stop traceway && docker rm traceway
```

---

## Notes

- Backend automatically runs database migrations on startup
- The minimal image starts in under 1 second
- For production, always use the minimal image with external ClickHouse for better reliability and scalability
