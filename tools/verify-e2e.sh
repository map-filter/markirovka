#!/bin/sh
set -e
cd "$(dirname "$0")/.."
docker image inspect markirovka-verify:dev >/dev/null 2>&1 || docker build -q -t markirovka-verify:dev -f tools/Dockerfile.verify tools
mkdir -p tools/out
docker run --rm --network host -u 1000:1000 \
    -v "$PWD/tools:/tools:ro" -v "$PWD/tools/out:/out" \
    --entrypoint python markirovka-verify:dev /tools/e2e.py
