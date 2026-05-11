DOCKER_COMPOSE = docker compose
PROTO_DIR      = proto

.PHONY: install
install:
	go install github.com/bufbuild/buf/cmd/buf@v1.67.0
	go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.1

.PHONY: proto
proto:
	buf generate

.PHONY: proto-lint
proto-lint:
	buf lint

.PHONY: format
format:
	@test -z "$$(gofmt -l ./src/backend/ ./src/frontend/telegram/)" || \
		(echo "❌ Unformatted files:"; gofmt -l ./src/backend/ ./src/frontend/telegram/; exit 1)

.PHONY: build
build:
	${DOCKER_COMPOSE} build

.PHONY: up
up:
	${DOCKER_COMPOSE} up -d --build

.PHONY: down
down:
	${DOCKER_COMPOSE} down

.PHONY: logs
logs:
	${DOCKER_COMPOSE} logs -f

.PHONY: migrate
migrate:
	${DOCKER_COMPOSE} exec backend sh -c 'echo "Migrations are applied automatically on backend startup"'

.PHONY: test
test:
	cd src/backend && go test ./...

.PHONY: coverage
coverage:
	cd src/backend && go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out
