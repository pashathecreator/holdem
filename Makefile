build:
	$(MAKE) -C services/auth-service build
	$(MAKE) -C services/engine build
	$(MAKE) -C services/table-manager build
	$(MAKE) -C services/wallet-service build
	$(MAKE) -C services/analytics-ingestor build

test:
	$(MAKE) -C services/auth-service test
	$(MAKE) -C services/engine test
	$(MAKE) -C services/table-manager test
	$(MAKE) -C services/wallet-service test
	$(MAKE) -C services/analytics-ingestor test

lint:
	$(MAKE) -C services/auth-service lint
	$(MAKE) -C services/engine lint
	$(MAKE) -C services/table-manager lint
	$(MAKE) -C services/wallet-service lint

integration:
	$(MAKE) -C services/engine integration

schemas-sync:
	$(MAKE) -C services/engine schemas-sync

auth-build:
	$(MAKE) -C services/auth-service build

auth-test:
	$(MAKE) -C services/auth-service test

wallet-build:
	$(MAKE) -C services/wallet-service build

wallet-test:
	$(MAKE) -C services/wallet-service test

table-manager-build:
	$(MAKE) -C services/table-manager build

table-manager-test:
	$(MAKE) -C services/table-manager test

analytics-build:
	$(MAKE) -C services/analytics-ingestor build

analytics-test:
	$(MAKE) -C services/analytics-ingestor test

run:
	docker compose up --build

rund:
	docker compose up --build -d

down:
	docker compose down

vdown:
	docker compose down -v
