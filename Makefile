# Optional convenience targets. The primary path is the self-bootstrapping
# app: `go run ./cmd/sheidan` provisions the GopherJS toolchain on first
# use, transpiles the client, and serves. These targets delegate to the
# sheidan CLI for the same operations.

.PHONY: web
web:
	go run ./cmd/sheidan web

.PHONY: test-web
test-web:
	go run ./cmd/sheidan test-web

.PHONY: run
run:
	go run ./cmd/sheidan
