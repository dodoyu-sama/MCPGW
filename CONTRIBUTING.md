# Contributing to MCPGW

Thanks for your interest in improving MCPGW! This guide covers local setup,
the build/test workflow, and how the repo is laid out.

## Prerequisites

- **Go 1.22+** with a C compiler (CGO) — required by the `mattn/go-sqlite3` driver.
  - macOS: `brew install go && xcode-select --install`
  - Linux: install `gcc`/`build-essential`. Windows is not tested.
- **Node 20+** — only needed to rebuild the embedded Vue3 console (`web/`).

## Build & test

```bash
make build     # builds web/dist, then compiles ./mcpgw
make run       # build + run the dev gateway (config.dev.yaml, SSE on :8081)
make test      # go test ./...
make clean     # remove the binary and web/dist
```

Run the gateway without rebuilding the frontend (console is embedded):

```bash
./mcpgw --config config.dev.yaml
# open http://localhost:8080
```

Live frontend development (Vite HMR on :5173, proxies `/api` → `:8080`):

```bash
./mcpgw --config config.dev.yaml &   # gateway + admin API
cd web && npm run dev
```

## Docker

```bash
docker compose up --build                              # SQLite (default), :8080
docker compose --profile postgres up --build           # PostgreSQL, gateway :8081
```

## Code layout

```
cmd/mcpgw/         entrypoint (flag parsing, wiring)
internal/config/   YAML config + env overrides + defaults
internal/proxy/    stdio/SSE transport, session routing, request/response passthrough
internal/audit/    audit store (SQLite + PostgreSQL), CallRecord model, migrate()
internal/mask/     regex + field-name masking presets and engine
internal/ratelimit/  per-client_id token bucket + daily quota
internal/admin/    REST API + embedded web console (web/embed.go)
web/               Vue3 dashboard (built into the binary via //go:embed)
```

## Configuration

All behaviour is driven by a YAML config (see `config.yaml` for the full schema
and defaults). `config.dev.yaml` / `config.postgres.yaml` / `config.sse*.yaml` are
ready-to-run examples. Local overrides can live in `config.local.yaml` (gitignored).

Audit backend: set `audit.driver: sqlite` (default, `dsn` = file path) or
`audit.driver: postgres` (`dsn` = Postgres connection URL).

## Guidelines

- Run `gofmt -w` (or `go fmt ./...`) and `go vet ./...` before opening a PR.
- Keep the single-binary, zero-external-dependency runtime promise: the web
  console is embedded, and the gateway runs with just a config file.
- Add/extend masking rules in `config.yaml` (`masking.rules`) rather than
  hardcoding them in callers.
- Open an issue before large changes so we can align on design.

## License

By contributing you agree that your contributions will be licensed under the
[MIT License](./LICENSE).
