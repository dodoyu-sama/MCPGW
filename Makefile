BINARY := mcp-arc

.PHONY: build web build-go run dev clean test

## Build the embedded web console, then compile the Go binary.
build: web build-go

web:
	cd web && npm install && npm run build

build-go:
	go build -o $(BINARY) ./cmd/mcp-arc

## Build + run the dev gateway (sqlite, SSE client on :8081, console on :8080).
run: build
	./$(BINARY) --config config.dev.yaml

## Dev-only frontend (Vite on :5173, proxies /api -> :8080). Needs a running gateway.
dev:
	cd web && npm run dev

clean:
	rm -f $(BINARY)
	rm -rf web/dist

test:
	go test ./...
