APP=passion

PORT ?= 3000
PASSION_ADDR ?= :$(PORT)

DB_PATH ?= passion.db

.PHONY: run run-v2 watch build reseed test test-all pg-up pg-down migrations catalog-lint catalog-ids

run:
	PASSION_ADDR="$(PASSION_ADDR)" go run ./cmd/passion

# V2. Runs the root main.go rather than cmd/passion.
#
# Supplies a throwaway secret and allows plain-HTTP cookies, so `make run-v2` works from a
# fresh clone with no config file. Both are local-dev only, and the binary already warns
# loudly on start when insecure cookies are on. A real deployment passes -config.
run-v2:
	PASSION_ADDR="$(PASSION_ADDR)" \
	PASSION_JWT_SECRET="local-dev-only-not-a-real-secret-000000" \
	PASSION_INSECURE_COOKIES=true \
	PASSION_DB_DSN="tmp/passion-v2.db" \
	go run .

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

# Checks whether the container is RUNNING, not merely whether it exists. `docker inspect`
# succeeds for an exited container, so the old test left a stopped container stopped and
# then waited 40s for a database that was never going to answer.
pg-up:
	@if [ "$$(docker inspect -f '{{.State.Running}}' $(PG_CONTAINER) 2>/dev/null)" = "true" ]; then \
		:; \
	elif docker inspect $(PG_CONTAINER) >/dev/null 2>&1; then \
		docker start $(PG_CONTAINER) >/dev/null; \
	else \
		docker run -d --name $(PG_CONTAINER) \
			-e POSTGRES_PASSWORD=passion -e POSTGRES_DB=passion_test \
			-p $(PG_PORT):5432 postgres:17-alpine >/dev/null; \
	fi
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

# Check the catalog trees without a database or a server. PRIVATE_CATALOG points at the
# other repository; the shipped tree comes first so its rows can be named with app:<slug>.
PRIVATE_CATALOG ?= ../passion-private-catalog
catalog-lint:
	go run . catalog lint catalog $(wildcard $(PRIVATE_CATALOG))

# Write an id into every catalog file that has none. Commit what it changes.
catalog-ids:
	go run . catalog lint --fix catalog $(wildcard $(PRIVATE_CATALOG))
