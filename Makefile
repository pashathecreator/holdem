build:
	go build ./cmd/server/...

run:
	docker compose up --build

rund:
	docker compose up --build -d

down:
	docker compose down

vdown:
	docker compose down -v