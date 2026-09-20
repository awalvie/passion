PGDATA ?= $(CURDIR)/.pgdata

IMAGE ?= passion:dev

.PHONY: db-up db-down image

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

image:
	docker build -t $(IMAGE) .
