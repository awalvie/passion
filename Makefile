PGDATA ?= $(CURDIR)/.pgdata

IMAGE ?= passion:dev

.PHONY: client db-up db-down image test

$(PGDATA):
	initdb --locale=C --encoding=UTF8 -D $(PGDATA)

# pg_ctl status exits 0 when running, 3 when stopped, 4 when there is no cluster.
db-up: $(PGDATA)
	@pg_ctl -D $(PGDATA) status >/dev/null 2>&1 || \
		pg_ctl -D $(PGDATA) -l $(PGDATA)/postgres.log -o "-k $(PGDATA) -h ''" -w start
	@psql -lqt | cut -d'|' -f1 | grep -qw passion || createdb passion
	@psql -lqt | cut -d'|' -f1 | grep -qw passion_test || createdb passion_test

db-down:
	@pg_ctl -D $(PGDATA) status >/dev/null 2>&1 && pg_ctl -D $(PGDATA) -w stop || true

# adapter-static empties its output directory, which takes .gitkeep with it.
# That file is what makes //go:embed match on a clone that has never built the
# client, so it has to come back.
client:
	pnpm --dir client install --frozen-lockfile
	pnpm --dir client build
	touch server/web/dist/.gitkeep

image:
	docker build -t $(IMAGE) .

# -p 1 runs one package at a time. Every package shares the one test database
# and empties it between tests, so two packages at once wipe each other.
test:
	go test ./... -count=1 -p 1
