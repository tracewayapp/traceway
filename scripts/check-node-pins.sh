#!/usr/bin/env bash
# Every Node version in this repo is derived from .nvmrc: the setup-node steps
# and flake.nix read it directly, and release-traceway.yml passes it to the
# Docker builds as NODE_VERSION. The Dockerfiles still carry an ARG default,
# because a bare `docker build` has no way to read .nvmrc -- so that default is
# the one value that can drift. This asserts it has not.
#
# Deliberately not checked: testing/devtesting-nestjs/Dockerfile, which pins the
# runtime of a sample third-party app rather than anything Traceway builds.
set -euo pipefail

cd "$(dirname "$0")/.."

pinned=$(tr -d '[:space:]v' < .nvmrc)
if [[ ! $pinned =~ ^[0-9]+$ ]]; then
	echo "check-node-pins: .nvmrc must hold a bare Node major, got '$pinned'" >&2
	exit 1
fi

status=0
for f in Dockerfile Dockerfile.minimal Dockerfile.sqlite Dockerfile.duckdb Dockerfile.browser; do
	found=$(sed -n 's/^ARG NODE_VERSION=\([0-9]*\).*/\1/p' "$f")
	if [[ -z $found ]]; then
		echo "check-node-pins: $f has no 'ARG NODE_VERSION=' default" >&2
		status=1
	elif [[ $found != "$pinned" ]]; then
		echo "check-node-pins: $f pins Node $found, .nvmrc says $pinned" >&2
		status=1
	fi
done

# A literal node: tag left behind would silently ignore both .nvmrc and the ARG.
if literal=$(grep -rnE '^FROM .*node:[0-9]' Dockerfile Dockerfile.minimal Dockerfile.sqlite Dockerfile.duckdb Dockerfile.browser); then
	echo "check-node-pins: hardcoded node tag, use \${NODE_VERSION}:" >&2
	echo "$literal" >&2
	status=1
fi

[[ $status -eq 0 ]] && echo "check-node-pins: all Dockerfiles agree with .nvmrc ($pinned)"
exit $status
