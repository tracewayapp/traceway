# Traceway Docker Deployment

Two deployment options:
1. **All-in-One** (`Dockerfile`) — ClickHouse + PostgreSQL + Backend in one container
2. **Minimal** (`Dockerfile.minimal`) — Lightweight Alpine image for external database deployments

---

## All-in-One Deployment

Single container with ClickHouse, PostgreSQL, and the Go backend managed by supervisord.

### Build

```bash
docker build -t traceway:latest .
```

### Run

```bash
docker run -d --name traceway \
  -p 80:80 \
  -v traceway-ch:/var/lib/clickhouse \
  -v traceway-pg:/var/lib/postgresql/data \
  traceway:latest
```

### Environment Variables

All have working defaults for the all-in-one image. Override as needed:

| Variable | Default | Description |
|----------|---------|-------------|
| `CLICKHOUSE_SERVER` | `localhost:9000` | ClickHouse host:port |
| `CLICKHOUSE_DATABASE` | `traceway` | ClickHouse database name |
| `CLICKHOUSE_USERNAME` | `default` | ClickHouse username |
| `CLICKHOUSE_PASSWORD` | *(empty)* | ClickHouse password |
| `CLICKHOUSE_TLS` | `false` | Enable TLS for ClickHouse |
| `POSTGRES_HOST` | `localhost` | PostgreSQL host |
| `POSTGRES_PORT` | `5432` | PostgreSQL port |
| `POSTGRES_DATABASE` | `traceway` | PostgreSQL database name |
| `POSTGRES_USERNAME` | `traceway` | PostgreSQL username |
| `POSTGRES_PASSWORD` | *(empty)* | PostgreSQL password |
| `POSTGRES_SSLMODE` | `disable` | PostgreSQL SSL mode |
| `JWT_SECRET` | *(built-in default)* | JWT signing secret (min 32 chars) |
| `GIN_MODE` | `release` | Gin framework mode |

### Access Points

| URL | Description |
|-----|-------------|
| `http://localhost/` | Frontend dashboard |
| `http://localhost/api/*` | Backend API |
| `http://localhost/health` | Health check |

---

## Minimal Deployment

Lightweight Alpine image (~20-30MB) containing only the Go backend with embedded frontend. Connect to external ClickHouse and PostgreSQL instances.

### Build

```bash
docker build -f Dockerfile.minimal -t traceway:minimal .
```

### Required Environment Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `CLICKHOUSE_SERVER` | ClickHouse host:port | `clickhouse:9000` |
| `CLICKHOUSE_DATABASE` | ClickHouse database | `traceway` |
| `CLICKHOUSE_USERNAME` | ClickHouse username | `default` |
| `CLICKHOUSE_PASSWORD` | ClickHouse password | `password` |
| `CLICKHOUSE_TLS` | Enable TLS | `false` |
| `POSTGRES_HOST` | PostgreSQL host | `postgres` |
| `POSTGRES_PORT` | PostgreSQL port | `5432` |
| `POSTGRES_DATABASE` | PostgreSQL database | `traceway` |
| `POSTGRES_USERNAME` | PostgreSQL username | `traceway` |
| `POSTGRES_PASSWORD` | PostgreSQL password | `password` |
| `POSTGRES_SSLMODE` | PostgreSQL SSL mode | `disable` |
| `JWT_SECRET` | JWT signing secret (min 32 chars) | `your-secret-here` |

### Run

```bash
docker run -d --name traceway \
  -p 80:80 \
  -e CLICKHOUSE_SERVER="clickhouse-host:9000" \
  -e CLICKHOUSE_DATABASE="traceway" \
  -e CLICKHOUSE_USERNAME="default" \
  -e CLICKHOUSE_PASSWORD="your-password" \
  -e POSTGRES_HOST="postgres-host" \
  -e POSTGRES_PORT="5432" \
  -e POSTGRES_DATABASE="traceway" \
  -e POSTGRES_USERNAME="traceway" \
  -e POSTGRES_PASSWORD="your-password" \
  -e POSTGRES_SSLMODE="disable" \
  -e JWT_SECRET="your-jwt-secret-min-32-characters-long" \
  traceway:minimal
```

### Docker Compose Example

```yaml
services:
  traceway:
    build:
      context: .
      dockerfile: Dockerfile.minimal
    ports:
      - "80:80"
    environment:
      CLICKHOUSE_SERVER: "clickhouse:9000"
      CLICKHOUSE_DATABASE: "traceway"
      CLICKHOUSE_USERNAME: "default"
      CLICKHOUSE_PASSWORD: "clickhouse"
      CLICKHOUSE_TLS: "false"
      POSTGRES_HOST: "postgres"
      POSTGRES_PORT: "5432"
      POSTGRES_DATABASE: "traceway"
      POSTGRES_USERNAME: "traceway"
      POSTGRES_PASSWORD: "traceway"
      POSTGRES_SSLMODE: "disable"
      JWT_SECRET: "change-this-to-a-secure-secret-at-least-32-chars"
    depends_on:
      clickhouse:
        condition: service_healthy
      postgres:
        condition: service_healthy
    restart: unless-stopped

  clickhouse:
    image: clickhouse/clickhouse-server:24.8-alpine
    environment:
      CLICKHOUSE_DB: traceway
      CLICKHOUSE_PASSWORD: clickhouse
    volumes:
      - clickhouse-data:/var/lib/clickhouse
    healthcheck:
      test: ["CMD", "clickhouse-client", "--password", "clickhouse", "--query", "SELECT 1"]
      interval: 5s
      timeout: 3s
      start_period: 10s
      retries: 10
    restart: unless-stopped

  postgres:
    image: postgres:17
    environment:
      POSTGRES_USER: traceway
      POSTGRES_PASSWORD: traceway
      POSTGRES_DB: traceway
    volumes:
      - postgres-data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U traceway"]
      interval: 5s
      timeout: 3s
      start_period: 10s
      retries: 10
    restart: unless-stopped

volumes:
  clickhouse-data:
  postgres-data:
```

---

## Docker Compose Quick Start

```bash
# Start the full stack (builds images and starts all services)
docker compose up --build

# Clean restart (removes volumes — resets all data)
docker compose down -v && docker compose up --build
```

After starting, open `http://localhost/register` to create your first account.

---

## Useful Commands

```bash
# View logs
docker logs traceway
docker logs -f traceway

# Enter container shell
docker exec -it traceway sh      # minimal (alpine)
docker exec -it traceway bash    # all-in-one (debian)

# Check process status (all-in-one only)
docker exec traceway supervisorctl status

# Health check
curl http://localhost/health

# Stop and remove
docker stop traceway && docker rm traceway
```
