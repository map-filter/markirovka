SHELL := /bin/sh
COMPOSE := docker compose
UID := 1000:1000
PHP_RUN := docker run --rm -u $(UID) -v $(CURDIR)/backend:/var/www/backend -e COMPOSER_HOME=/tmp/composer markirovka-php:dev
GO_RUN := docker run --rm -u $(UID) -e CGO_ENABLED=0 -e GOFLAGS=-buildvcs=false -e GOCACHE=/tmp/gocache -e GOPATH=/tmp/go -v $(CURDIR):/src golang:1.23-alpine
NODE_RUN := docker run --rm -u $(UID) -v $(CURDIR)/frontend:/app -w /app -e npm_config_cache=/app/.npm-cache node:22-alpine

.PHONY: env up down restart reset logs ps seed shell test test-php test-go test-front verify

env:
	@sh tools/make-env.sh

up: env
	$(COMPOSE) build
	$(COMPOSE) up -d --wait
	@echo "Кабинет:   http://127.0.0.1:8301"
	@echo "API:       http://127.0.0.1:8301/docs/api/"
	@echo "Эмулятор:  http://127.0.0.1:8303/emu/"

down:
	$(COMPOSE) down

restart: down up

reset: env
	$(COMPOSE) down -v
	$(MAKE) up

logs:
	$(COMPOSE) logs -f --tail=100

ps:
	$(COMPOSE) ps

seed:
	curl -fsS -X POST http://127.0.0.1:8303/emu/reset >/dev/null
	$(COMPOSE) exec markirovka-app php artisan migrate:fresh --seed --force

shell:
	$(COMPOSE) exec markirovka-app sh

test: test-php test-go test-front

test-php: env
	$(COMPOSE) build markirovka-app
	$(COMPOSE) up -d --wait markirovka-db
	@test -f backend/vendor/autoload.php || $(PHP_RUN) composer install --no-interaction --no-progress
	docker run --rm -u $(UID) --network markirovka_default -v $(CURDIR)/backend:/var/www/backend \
		-v $(CURDIR)/seed:/seed:ro --env-file .env -e APP_ENV=testing -e DB_CONNECTION=pgsql -e DB_HOST=markirovka-db \
		-e DB_DATABASE=markirovka_test -e SEED_FILE=/seed/seed.json markirovka-php:dev php artisan test

test-go:
	$(GO_RUN) sh -c "cd /src/emulator && go vet ./... && go test ./... && cd /src/agent && go vet ./... && go test ./..."

test-front:
	$(NODE_RUN) sh -c "npm ci --no-audit --no-fund --loglevel=error && npm run typecheck"

verify:
	sh tools/verify-e2e.sh
