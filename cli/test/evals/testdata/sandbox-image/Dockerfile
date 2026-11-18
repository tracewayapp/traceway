# Sandbox for the setup suite's shell and for running hidden tests against a
# model's patch. Toolchains only: no credentials ever enter this image.
# NODE_VERSION follows the repo's .nvmrc (see scripts/check-node-pins.sh);
# GO_VERSION follows cli/test/evals/go.mod.
ARG NODE_VERSION=26
ARG GO_VERSION=1.26
FROM golang:${GO_VERSION}-bookworm AS go

FROM node:${NODE_VERSION}-bookworm
COPY --from=go /usr/local/go /usr/local/go
ENV PATH=/usr/local/go/bin:$PATH
RUN apt-get update \
 && apt-get install -y --no-install-recommends python3 python3-pip python3-venv curl git jq ca-certificates \
 && rm -rf /var/lib/apt/lists/* \
 && git config --system user.email "dev@example.com" \
 && git config --system user.name "Dana Developer" \
 && git config --system --add safe.directory '*'
ENV GOTOOLCHAIN=local GOFLAGS=-mod=mod PIP_BREAK_SYSTEM_PACKAGES=1 npm_config_update_notifier=false
WORKDIR /work
