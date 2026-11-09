# Lokasi binary migrate. Default: folder bin milik Go, jadi tidak bergantung pada PATH.
MIGRATE ?= $(shell go env GOPATH)/bin/migrate
# Lokasi binary Air (restart otomatis untuk make dev), juga di folder bin milik Go.
AIR ?= $(shell go env GOPATH)/bin/air

# Create a set of database up/down migrations with a specific name.
migrate-create:
# -ext adalah file extension, artinya kita membuat file.sql || -dir adalah folder tempat simpan || usahakan jangan menggunakan spasi dalam penamaan file migration
	$(MIGRATE) create -ext sql -dir db/migrations ${APP_DB_NAME}

# Migrate database up
migrate-up:
	@echo "Migrating database up..."
	$(MIGRATE) -path db/migrations -database "postgresql://$(APP_DB_USER):$(APP_DB_PASSWORD)@$(APP_DB_HOST):$(APP_DB_PORT)/$(APP_DB_NAME)?sslmode=disable" -verbose up
	@echo "Migration up completed successfully."

# Migrate database down
migrate-down:
	@echo "Migrating database down..."
	$(MIGRATE) -path db/migrations -database "postgresql://$(APP_DB_USER):$(APP_DB_PASSWORD)@$(APP_DB_HOST):$(APP_DB_PORT)/$(APP_DB_NAME)?sslmode=disable" -verbose down
	@echo "Migration down completed successfully."

# Buat admin pertama: make seed-admin USERNAME=admin NAME="Admin Toko"
seed-admin:
	go run ./cmd/seed-admin --username "$(USERNAME)" --name "$(NAME)"

scan-vuln-general:
	osv-scanner -r go.mod

scan-vuln-with-json-output:
	osv-scanner --lockfile=go.mod --config=osv-scanner.toml --format json > results.json

scan-vuln-general-with-config:
	osv-scanner --lockfile=go.mod --config=osv-scanner.toml --format=table

run:
	go run main.go

# Seperti run, tapi server dibangun dan dijalankan ulang otomatis setiap ada file .go yang berubah (Air, pengaturan
# di .air.toml). Pasang Air sekali: go install github.com/air-verse/air@latest
dev:
	@test -x "$(AIR)" || { echo "Air belum terpasang: go install github.com/air-verse/air@latest"; exit 1; }
	$(AIR)

# Tes tanpa database (tes yang butuh database dilewati).
test:
	go test ./...

# Tes lengkap: database percobaan pos_check dibuat ulang dari migrasi, lalu semua tes dijalankan di sana.
# Database toko (APP_DB_NAME di env.sh) tidak disentuh.
test-db:
	@PGPASSWORD="$(APP_DB_PASSWORD)" dropdb -h $(APP_DB_HOST) -p $(APP_DB_PORT) -U $(APP_DB_USER) --if-exists pos_check
	@PGPASSWORD="$(APP_DB_PASSWORD)" createdb -h $(APP_DB_HOST) -p $(APP_DB_PORT) -U $(APP_DB_USER) pos_check
	@$(MIGRATE) -path db/migrations -database "postgresql://$(APP_DB_USER):$(APP_DB_PASSWORD)@$(APP_DB_HOST):$(APP_DB_PORT)/pos_check?sslmode=disable" up
	APP_DB_NAME=pos_check go test -count=1 ./...

.PHONY: migrate-create migrate-up migrate-down seed-admin scan-vuln-general scan-vuln-with-json-output scan-vuln-general-with-config run dev test test-db
include env.sh