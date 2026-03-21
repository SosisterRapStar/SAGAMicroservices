BOOKING_DIR    := BookingMicroservice
FLIGHTS_DIR    := flightsMicro
HOTELS_DIR     := hotelMicro

BOOKING_MIGRATE_DSN  := postgres://postgres:postgres@localhost:5432/booking_db?sslmode=disable
FLIGHTS_MIGRATE_DSN  := postgres://postgres:postgres@localhost:5435/flights_db?sslmode=disable&search_path=flight
HOTELS_MIGRATE_DSN   := mysql://mysql:mysql@tcp(localhost:3306)/hotels_db

.PHONY: help up infra migrate migrate-booking migrate-flights migrate-hotels run run-booking run-flights run-hotels down

help:
	@echo "Targets:"
	@echo "  make up              — поднять инфраструктуру + накатить миграции"
	@echo "  make infra           — поднять только контейнеры (Postgres, MySQL, Kafka, Jaeger)"
	@echo "  make migrate         — накатить миграции для всех сервисов"
	@echo "  make run             — запустить все три сервиса"
	@echo "  make down            — остановить контейнеры"

infra:
	docker compose up -d
	@echo "Waiting for Postgres..."
	@until docker exec postgres-booking pg_isready -U postgres -d booking_db > /dev/null 2>&1; do sleep 1; done
	@echo "Waiting for Postgres (flights)..."
	@until docker exec postgres-flights pg_isready -U postgres -d flights_db > /dev/null 2>&1; do sleep 1; done
	@echo "Waiting for MySQL..."
	@until docker exec mysql mysqladmin ping -h localhost --silent > /dev/null 2>&1; do sleep 1; done
	@echo "Waiting for Kafka..."
	@sleep 5
	@echo "Infrastructure is ready."

migrate-booking:
	MIGRATE_DSN="$(BOOKING_MIGRATE_DSN)" $(MAKE) -C $(BOOKING_DIR) migration-up

migrate-flights:
	MIGRATE_DSN="$(FLIGHTS_MIGRATE_DSN)" $(MAKE) -C $(FLIGHTS_DIR) migration-up

migrate-hotels:
	MIGRATE_DSN="$(HOTELS_MIGRATE_DSN)" $(MAKE) -C $(HOTELS_DIR) migration-up

migrate: migrate-booking migrate-flights migrate-hotels

up: infra migrate

run-booking:
	$(MAKE) -C $(BOOKING_DIR) run

run-flights:
	$(MAKE) -C $(FLIGHTS_DIR) run

run-hotels:
	$(MAKE) -C $(HOTELS_DIR) run

run:
	$(MAKE) run-booking & \
	$(MAKE) run-flights & \
	$(MAKE) run-hotels & \
	wait

down:
	docker compose down
