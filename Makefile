GO ?= go

.PHONY: fmt test test-compose build

fmt:
	$(GO) fmt ./...

test:
	$(GO) test -race ./...
	$(GO) vet ./...

test-compose:
	@set -eu; \
	project="psono-terraform-provider-test-$$(od -An -N16 -tx1 /dev/urandom | tr -d ' ')"; \
	cleanup() { status=$$?; trap - EXIT INT TERM; docker compose -p "$$project" -f compose.test.yaml down --volumes --remove-orphans --rmi local >/dev/null 2>&1 || true; exit "$$status"; }; \
	on_signal() { trap - INT TERM; exit "$$1"; }; \
	trap cleanup EXIT; \
	trap 'on_signal 130' INT; \
	trap 'on_signal 143' TERM; \
	docker compose -p "$$project" -f compose.test.yaml up --build --abort-on-container-exit --exit-code-from integration

build:
	$(GO) build -trimpath -o bin/terraform-provider-psono ./cmd/terraform-provider-psono
