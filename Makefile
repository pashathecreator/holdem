build:
	go build ./cmd/server/...

lint:
	$(MAKE) -C services/engine lint

integration:
	$(MAKE) -C services/engine integration

schemas-sync:
	$(MAKE) -C services/engine schemas-sync

run:
	docker compose up --build

rund:
	docker compose up --build -d

down:
	docker compose down

vdown:
	docker compose down -v
