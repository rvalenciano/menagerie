.PHONY: test test-race build up down logs bench bench-down clean

test:
	go test ./...

test-race:
	go test -race ./...

build:
	go build ./...

up:
	docker compose up --build -d

down:
	docker compose down

logs:
	docker compose logs -f

# Usage: make bench RATE=1000 DURATION=1m
bench:
	docker compose -f docker-compose.yml -f docker-compose.bench.yml up --build --abort-on-container-exit vegeta

bench-down:
	docker compose -f docker-compose.yml -f docker-compose.bench.yml down

clean: down
	go clean
