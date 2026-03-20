APP=passion

PORT ?= 3000
PASSION_ADDR ?= :$(PORT)

DB_PATH ?= passion.db

.PHONY: run watch build reseed

run:
	PASSION_ADDR="$(PASSION_ADDR)" go run ./cmd/passion

build:
	go build ./...

watch:
	@command -v air >/dev/null 2>&1 || go install github.com/air-verse/air@latest
	PASSION_ADDR="$(PASSION_ADDR)" air -c .air.toml

reseed:
	@echo "Deleting $(DB_PATH) and reseeding..."
	@rm -f $(DB_PATH)
	PASSION_ADDR="$(PASSION_ADDR)" go run ./cmd/passion --exit-after-seed

