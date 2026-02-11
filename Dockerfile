# ==============================================================================
# Traceway All-in-One Docker Image
# Includes: Frontend (embedded), Backend (Go), ClickHouse, PostgreSQL
# Managed by supervisord
# ==============================================================================

# ==============================================================================
# Stage 1: Build Frontend
# ==============================================================================
FROM node:22-alpine AS frontend-builder

WORKDIR /app/frontend

COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

COPY frontend/ ./

ENV CLOUD_MODE=false
RUN npm run build

# ==============================================================================
# Stage 2: Build Backend with embedded frontend
# ==============================================================================
FROM golang:1.25-alpine AS backend-builder

WORKDIR /app/backend

RUN apk add --no-cache git

COPY backend/ ./
COPY --from=frontend-builder /app/frontend/build ./static/dist/

RUN go mod edit -dropreplace=go.tracewayapp.com -dropreplace=go.tracewayapp.com/tracewaygin
RUN go mod tidy
RUN go mod download

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /traceway .

# ==============================================================================
# Stage 3: ClickHouse binary source
# ==============================================================================
FROM clickhouse/clickhouse-server:24.8-alpine AS clickhouse-source

# ==============================================================================
# Stage 4: Runtime image
# ==============================================================================
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    supervisor \
    ca-certificates \
    curl \
    musl \
    postgresql \
    postgresql-client \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /var/log/supervisor \
    && mkdir -p /var/lib/clickhouse \
    && mkdir -p /var/log/clickhouse-server \
    && mkdir -p /etc/clickhouse-server/config.d \
    && mkdir -p /etc/clickhouse-server/users.d

# ClickHouse binary + symlinks
COPY --from=clickhouse-source /usr/bin/clickhouse /usr/bin/clickhouse
COPY --from=clickhouse-source /etc/clickhouse-server/config.xml /etc/clickhouse-server/config.xml
COPY --from=clickhouse-source /etc/clickhouse-server/users.xml /etc/clickhouse-server/users.xml

RUN ln -s /usr/bin/clickhouse /usr/bin/clickhouse-server \
    && ln -s /usr/bin/clickhouse /usr/bin/clickhouse-client

# Go backend binary
COPY --from=backend-builder /traceway /usr/local/bin/traceway

# Configuration files
COPY docker/supervisord.conf /etc/supervisor/supervisord.conf
COPY docker/clickhouse-config.xml /etc/clickhouse-server/config.d/docker.xml
COPY docker/clickhouse-users.xml /etc/clickhouse-server/users.d/docker.xml
COPY docker/wait-for-clickhouse.sh /usr/local/bin/wait-for-clickhouse.sh
COPY docker/start-backend.sh /usr/local/bin/start-backend.sh
COPY docker/init-postgres.sh /usr/local/bin/init-postgres.sh

RUN chmod +x /usr/local/bin/wait-for-clickhouse.sh \
    && chmod +x /usr/local/bin/start-backend.sh \
    && chmod +x /usr/local/bin/init-postgres.sh

# Create clickhouse user and set permissions
RUN useradd -r -s /bin/false clickhouse \
    && chown -R clickhouse:clickhouse /var/lib/clickhouse \
    && chown -R clickhouse:clickhouse /var/log/clickhouse-server

WORKDIR /app
COPY backend/.env.docker /app/.env

ENV GIN_MODE=release

VOLUME ["/var/lib/clickhouse", "/var/lib/postgresql/data"]

EXPOSE 80 8082

HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
    CMD curl -f http://localhost/health || exit 1

CMD ["/usr/bin/supervisord", "-c", "/etc/supervisor/supervisord.conf"]
