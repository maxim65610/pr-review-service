build:
	go build -o pr-service ./cmd/app

run:
	DATABASE_DSN="postgres://postgres:postgres@localhost:5432/prservice?sslmode=disable" \
	go run ./cmd/app

docker-up:
	docker-compose up --build

docker-down:
	docker-compose down

test-e2e:
	docker compose -f docker-compose.e2e.yml up --build --abort-on-container-exit


