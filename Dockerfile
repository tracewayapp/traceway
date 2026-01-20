# ==============================================================================
# Traceway Standalone Docker Image
# Single binary serving both API and Frontend (no ClickHouse included)
# Uses systemd for process management with watchdog support
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
# Stage 2: Build Backend with embedded frontend
# ==============================================================================
FROM golang:1.25-alpine AS backend-builder

WORKDIR /app/backend

# Install build dependencies
RUN apk add --no-cache git

# Copy all backend source
COPY backend/ ./

# Copy built frontend to static/dist for embedding
COPY --from=frontend-builder /app/frontend/build ./static/dist/

# Download dependencies
RUN go mod download

# Build with embedded static files
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /traceway .

# ==============================================================================
# Stage 3: Final runtime image with systemd
# ==============================================================================
FROM debian:bookworm-slim

# Install systemd and minimal dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    systemd \
    systemd-sysv \
    ca-certificates \
    curl \
    && rm -rf /var/lib/apt/lists/* \
    # Remove unnecessary systemd services
    && rm -f /lib/systemd/system/multi-user.target.wants/* \
    && rm -f /etc/systemd/system/*.wants/* \
    && rm -f /lib/systemd/system/local-fs.target.wants/* \
    && rm -f /lib/systemd/system/sockets.target.wants/*udev* \
    && rm -f /lib/systemd/system/sockets.target.wants/*initctl* \
    && rm -f /lib/systemd/system/basic.target.wants/* \
    && rm -f /lib/systemd/system/anaconda.target.wants/*

# Copy the binary
COPY --from=backend-builder /traceway /usr/local/bin/traceway

# Create app directory
RUN mkdir -p /app

# Create systemd service file for traceway
RUN cat > /etc/systemd/system/traceway.service << 'EOF'
[Unit]
Description=Traceway Error Tracking Server
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/traceway
WorkingDirectory=/app
Restart=always
RestartSec=5

# Environment variables
Environment=ENABLE_PORT_80=true
Environment=GIN_MODE=release

# Watchdog - service must notify within 30s or it will be restarted
WatchdogSec=30

# Security hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/app

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=traceway

[Install]
WantedBy=multi-user.target
EOF

# Enable the service
RUN systemctl enable traceway.service

# Set working directory
WORKDIR /app

# Environment variables (can be overridden at runtime)
ENV ENABLE_PORT_80=true
ENV GIN_MODE=release

# Expose ports
EXPOSE 80 8082

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
    CMD curl -f http://localhost/health || exit 1

# Use systemd as init
STOPSIGNAL SIGRTMIN+3
CMD ["/lib/systemd/systemd"]
