FROM golang:1.25-alpine

RUN apk add --no-cache git

WORKDIR /app/devtesting

# Default endpoint for macOS (host.docker.internal reaches the Mac host)
# Override with -e TRACEWAY_ENDPOINT=... for different setups
ENV TRACEWAY_ENDPOINT="default_token_change_me@http://host.docker.internal:8082/api/report"

CMD ["tail", "-f", "/dev/null"]
