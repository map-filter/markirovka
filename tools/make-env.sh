#!/bin/sh
set -e
cd "$(dirname "$0")/.."
mkdir -p var/prints
if [ -f .env ]; then
    exit 0
fi
random() {
    head -c 48 /dev/urandom | base64 | tr -dc 'A-Za-z0-9' | head -c "$1"
}
key="base64:$(head -c 32 /dev/urandom | base64)"
sed -e "s|^APP_KEY=.*|APP_KEY=${key}|" \
    -e "s|^DB_PASSWORD=.*|DB_PASSWORD=$(random 24)|" \
    -e "s|^AGENT_TOKEN=.*|AGENT_TOKEN=mkag_$(random 40)|" \
    .env.example > .env
echo "Создан .env со случайными паролями"
