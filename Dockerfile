# ==============================================================================
# Traceway All-in-One Docker Image
# Includes: Frontend (SvelteKit), Backend (Go), ClickHouse
# ==============================================================================

# ==============================================================================
# Stage 1: Build Frontend
# ==============================================================================
FROM node:22-alpine AS frontend-builder

WORKDIR /app/frontend

# Copy package files first for better caching
COPY frontend/package.json frontend/package-lock.json ./

# Install dependencies
RUN npm ci

# Copy source and build
COPY frontend/ ./
RUN npm run build

# ==============================================================================
# Stage 2: Build Backend
# ==============================================================================
FROM golang:1.24-alpine AS backend-builder

WORKDIR /app/backend

# Install build dependencies
RUN apk add --no-cache git

# Copy all backend source
COPY backend/ ./

# Fix go version (1.25.1 doesn't exist yet, use 1.24) and download deps
RUN go mod edit -go=1.24 && go mod download

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /traceway-backend .

# ==============================================================================
# Stage 3: Extract ClickHouse binaries
# ==============================================================================
FROM clickhouse/clickhouse-server:24.8-alpine AS clickhouse-source

# ==============================================================================
# Stage 4: Final Runtime Image
# ==============================================================================
FROM debian:bookworm-slim

# Install runtime dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    nginx \
    supervisor \
    ca-certificates \
    curl \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /var/log/supervisor \
    && mkdir -p /var/log/nginx \
    && mkdir -p /var/lib/clickhouse \
    && mkdir -p /var/log/clickhouse-server \
    && mkdir -p /etc/clickhouse-server/config.d \
    && mkdir -p /etc/clickhouse-server/users.d

# Copy ClickHouse binaries and configs from official image
COPY --from=clickhouse-source /usr/bin/clickhouse /usr/bin/clickhouse
COPY --from=clickhouse-source /etc/clickhouse-server/config.xml /etc/clickhouse-server/config.xml
COPY --from=clickhouse-source /etc/clickhouse-server/users.xml /etc/clickhouse-server/users.xml

# Create ClickHouse symlinks (clickhouse is a multicall binary)
RUN ln -s /usr/bin/clickhouse /usr/bin/clickhouse-server \
    && ln -s /usr/bin/clickhouse /usr/bin/clickhouse-client

# Copy built frontend
COPY --from=frontend-builder /app/frontend/build /var/www/html

# Copy built backend
COPY --from=backend-builder /traceway-backend /usr/local/bin/traceway-backend

# Copy configuration files
COPY docker/nginx.conf /etc/nginx/nginx.conf
COPY docker/supervisord.conf /etc/supervisor/conf.d/supervisord.conf
COPY docker/clickhouse-config.xml /etc/clickhouse-server/config.d/docker.xml
COPY docker/clickhouse-users.xml /etc/clickhouse-server/users.d/docker.xml
COPY docker/wait-for-clickhouse.sh /usr/local/bin/wait-for-clickhouse.sh
COPY docker/start-backend.sh /usr/local/bin/start-backend.sh

# Make scripts executable
RUN chmod +x /usr/local/bin/wait-for-clickhouse.sh \
    && chmod +x /usr/local/bin/start-backend.sh

# Create clickhouse user and set permissions
RUN useradd -r -s /bin/false clickhouse \
    && chown -R clickhouse:clickhouse /var/lib/clickhouse \
    && chown -R clickhouse:clickhouse /var/log/clickhouse-server

# Create app directory for backend and copy env file
WORKDIR /app
COPY backend/.env.docker /app/.env

# Expose ports
# Port 80: nginx (frontend + API proxy)
# Port 8082: direct backend access (optional)
EXPOSE 80 8082

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
    CMD curl -f http://localhost/health || exit 1

# Start supervisord
CMD ["/usr/bin/supervisord", "-c", "/etc/supervisor/conf.d/supervisord.conf"]
