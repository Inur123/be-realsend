.PHONY: run worker build migrate-up migrate-down seed clean tidy

# ── Development ──────────────────────────────────────────
run:
	go run cmd/api/main.go

worker:
	go run cmd/worker/main.go

# ── Build ────────────────────────────────────────────────
build:
	go build -o bin/api cmd/api/main.go
	go build -o bin/worker cmd/worker/main.go

# ── Database ─────────────────────────────────────────────
DB_URL ?= postgres://realsend_user:realsend_secret@localhost:5432/realsend?sslmode=disable

# Run all migrations in order
migrate-up:
	@echo "🔄 Running migrations..."
	@for f in $$(ls migrations/*.up.sql | sort); do \
		echo "  ⬆️  $$f"; \
		psql "$(DB_URL)" -f "$$f"; \
	done
	@echo "✅ All migrations applied!"

# Rollback all migrations in reverse order
migrate-down:
	@echo "🔄 Rolling back migrations..."
	@for f in $$(ls migrations/*.down.sql | sort -r); do \
		echo "  ⬇️  $$f"; \
		psql "$(DB_URL)" -f "$$f"; \
	done
	@echo "✅ All migrations rolled back!"

# Seed default data
seed:
	@echo "🌱 Seeding data..."
	psql "$(DB_URL)" -f migrations/011_seed_plans.up.sql
	@echo "✅ Seed complete!"

# ── Go Module ────────────────────────────────────────────
tidy:
	go mod tidy

# ── Cleanup ──────────────────────────────────────────────
clean:
	rm -rf bin/

# ── Database Setup (first time) ──────────────────────────
db-create:
	@echo "🗄️  Creating database..."
	psql postgres -c "CREATE USER realsend_user WITH PASSWORD 'realsend_secret';" || true
	psql postgres -c "CREATE DATABASE realsend OWNER realsend_user;" || true
	psql realsend -c "GRANT ALL ON SCHEMA public TO realsend_user;" || true
	@echo "✅ Database created!"

db-drop:
	@echo "⚠️  Dropping database..."
	psql postgres -c "DROP DATABASE IF EXISTS realsend;"
	psql postgres -c "DROP USER IF EXISTS realsend_user;"
	@echo "✅ Database dropped!"

db-reset: db-drop db-create migrate-up
	@echo "✅ Database reset complete!"
