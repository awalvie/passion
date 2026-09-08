APP=passion

PORT ?= 3000
PASSION_ADDR ?= :$(PORT)

DB_PATH ?= passion.db

.PHONY: run run-v2 watch build reseed test test-all pg-up pg-down migrations

run:
	PASSION_ADDR="$(PASSION_ADDR)" go run ./cmd/passion

# V2. Runs the root main.go rather than cmd/passion.
run-v2:
	PASSION_ADDR="$(PASSION_ADDR)" go run .

build:
	go build ./...

watch:
	@command -v air >/dev/null 2>&1 || go install github.com/air-verse/air@latest
	PASSION_ADDR="$(PASSION_ADDR)" air -c .air.toml

reseed:
	@echo "Deleting $(DB_PATH) and reseeding..."
	@rm -f $(DB_PATH)
	PASSION_ADDR="$(PASSION_ADDR)" go run ./cmd/passion --exit-after-seed


# ---------------------------------------------------------------------------
# V2 targets
# ---------------------------------------------------------------------------

PG_CONTAINER ?= passion-test-pg
PG_PORT      ?= 55432
PG_DSN       ?= postgres://postgres:passion@localhost:$(PG_PORT)/passion_test?sslmode=disable

# SQLite only. Fast, and what you run while writing code.
test:
	go test ./store/... ./config/... ./web/... -count=1

# Both engines. Needs pg-up first. Every store test runs twice, because deleting an
# account behaved differently per engine from identical DDL and a SQLite-only run
# would not have caught it.
test-all: pg-up
	PASSION_TEST_POSTGRES="$(PG_DSN)" go test ./store/... ./config/... ./web/... -count=1

pg-up:
	@docker inspect $(PG_CONTAINER) >/dev/null 2>&1 || \
		docker run -d --name $(PG_CONTAINER) \
			-e POSTGRES_PASSWORD=passion -e POSTGRES_DB=passion_test \
			-p $(PG_PORT):5432 postgres:17-alpine >/dev/null
	@printf 'waiting for postgres'
	@for i in $$(seq 1 40); do \
		docker exec $(PG_CONTAINER) psql -U postgres -d passion_test -c 'SELECT 1' >/dev/null 2>&1 && \
			{ echo " ready"; exit 0; }; \
		printf '.'; sleep 1; \
	done; echo " timed out"; exit 1

pg-down:
	@docker rm -f $(PG_CONTAINER) >/dev/null 2>&1 || true

# Both dialect migrations are generated from docs/SCHEMA_V2.sql so they cannot drift.
migrations:
	go run ./cmd/genmigrations
