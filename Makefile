# FlowForge reproducible tool commands.
# All targets run from the repository root.

WEB_DIR := apps/web

.PHONY: setup fmt lint test build up down verify clean

## setup: install all local dependencies (Go modules + pnpm packages)
setup:
	go mod download
	cd $(WEB_DIR) && pnpm install --frozen-lockfile

## fmt: format Go and frontend sources
fmt:
	gofmt -w .
	cd $(WEB_DIR) && pnpm exec prettier --write . 2>/dev/null || true

## lint: run Go vet and frontend ESLint
lint:
	go vet ./...
	cd $(WEB_DIR) && pnpm lint

## test: run Go tests (incl. race detector) and frontend unit tests
test:
	go test ./...
	go test -race ./...
	cd $(WEB_DIR) && pnpm test

## build: compile Go binaries and the Next.js production build
build:
	go build ./...
	cd $(WEB_DIR) && pnpm exec tsc --noEmit
	cd $(WEB_DIR) && pnpm build

## up: boot the full local topology (added in phase 15 compose file)
up:
	docker compose -f infra/docker-compose.yml up -d --build

## down: stop the local topology
down:
	docker compose -f infra/docker-compose.yml down

## verify: full local verification gate (format check, lint, tests, builds)
verify:
	test -z "$$(gofmt -l .)" || (echo "gofmt: files need formatting" && gofmt -l . && exit 1)
	go vet ./...
	go test ./...
	go test -race ./...
	go build ./...
	cd $(WEB_DIR) && pnpm lint
	cd $(WEB_DIR) && pnpm exec tsc --noEmit
	cd $(WEB_DIR) && pnpm build
	git diff --check

## clean: remove local build artifacts
clean:
	rm -rf $(WEB_DIR)/.next bin dist
