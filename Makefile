################################
# Dependency related commands
################################
.PHONY: dependency
dependency: install-golangci-lint install-mockgen install-goimports

.PHONY: install-mockgen
install-mockgen:
	go install go.uber.org/mock/mockgen@v0.5.2

.PHONY: install-golangci-lint
install-golangci-lint:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.2.2

.PHONY: install-goimports
install-goimports:
	go install golang.org/x/tools/cmd/goimports@v0.35.0

################################
# CI related commands
################################
.PHONY: fix-imports
fix-imports:
	@goimports -w ./cmd
	@goimports -w ./internal
	@goimports -w ./pkg

.PHONY: lint
lint:
	./scripts/lint.sh

.PHONY: test
test:
	./scripts/test.sh

.PHONY: install-hooks
install-hooks:
	cp .hooks/pre-commit .git/hooks/pre-commit
	sudo chmod +x .git/hooks/pre-commit

.PHONY: uninstall-hooks
uninstall-hooks:
	rm .git/hooks/pre-commit

.PHONY: generate
generate:
	go generate ./...

################################
# Dev environment related commands
################################
.PHONY: start-docker-compose
start-docker-compose:
	docker compose up -d

.PHONY: stop-docker-compose
stop-docker-compose:
	docker compose stop

.PHONY: remove-docker-compose
remove-docker-compose:
	docker compose down -v || true

.PHONY: clean
clean: remove-docker-compose

################################
# Migration related commands
################################
.PHONY: create-migration
create-migration:
ifneq ($(name),)
	goose -s create $(name) sql
else
	@echo "Migration name not specified. Usage: make create-migration name=<NAME>"
endif


.PHONY: migrate-up
migrate-up:
	go run cmd/* -c config.yml migrate