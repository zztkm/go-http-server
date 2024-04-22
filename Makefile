sqlite3def := go run github.com/sqldef/sqldef/cmd/sqlite3def@v0.17.6
sqlc := go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.26.0


.PHONY: gen
gen:
	$(sqlc) generate

.PHONY: migrate-dry
migrate-dry:
	$(sqlite3def) -f schema.sql --dry-run db.sqlite

.PHONY: migrate
migrate:
	$(sqlite3def) -f schema.sql db.sqlite

.PHONY: dev
dev:
	go run ./main.go

.PHONY: build
build:
	go build .

