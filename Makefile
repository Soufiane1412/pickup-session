DSN ?= postgres://pickup:pickup@localhost:5433/pickup?sslmode=disable

db-up:      ; docker compose up -d db
db-reset:   ; docker compose down -v && docker compose up -d db
test:       ; go test ./... -count=1
test-pg:    ; PICKUP_DSN="$(DSN)" go test ./... -count=1 -run Postgres
