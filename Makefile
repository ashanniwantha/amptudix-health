# Load environment variables from .env if present
ifneq (,$(wildcard .env))
    include .env
    export
endif

.PHONY: help up down clean restart logs ps \
        migrate-up migrate-down migrate-status migrate-version \
        shell-db shell-redis

# Default target: print available commands
help:
	@echo "Usage: make <target>"
	@echo ""
	@echo "Lifecycle:"
	@echo "  up               Build and start the full stack"
	@echo "  down             Stop the stack (keep volumes)"
	@echo "  clean            Stop the stack AND delete Postgres/Redis volumes"
	@echo "  restart          Restart only the API container"
	@echo ""
	@echo "Observability:"
	@echo "  logs             Tail API logs"
	@echo "  ps               Show container status"
	@echo ""
	@echo "Migrations:"
	@echo "  migrate-up       Apply all pending migrations"
	@echo "  migrate-down     Roll back the most recent migration"
	@echo "  migrate-status   Show applied/pending migrations"
	@echo "  migrate-version  Print the current schema version"
	@echo ""
	@echo "Shells:"
	@echo "  shell-db         psql shell on the Postgres container"
	@echo "  shell-redis      redis-cli shell on the Redis container"

up:
	docker compose up -d --build

down:
	docker compose down

clean:
	docker compose down -v

restart:
	docker compose restart api

logs:
	docker compose logs -f api

ps:
	docker compose ps

# Migrations run in a one-off container. The "migrate" service in
# compose.yaml already defaults to "up", but passing the command explicitly
# keeps the Makefile readable.
migrate-up:
	docker compose run --rm migrate up

migrate-down:
	docker compose run --rm migrate down

migrate-status:
	docker compose run --rm migrate status

migrate-version:
	docker compose run --rm migrate version

# Postgres container uses POSTGRES_USER / POSTGRES_DB, not DB_USER / DB_NAME.
shell-db:
	docker compose exec postgres psql -U $(POSTGRES_USER) -d $(POSTGRES_DB)

shell-redis:
	docker compose exec redis redis-cli
	