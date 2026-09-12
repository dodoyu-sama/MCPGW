[English](./README.md) · [中文](./README.zh-CN.md)

# MCPGW

A transparent proxy / sidecar for the [Model Context Protocol (MCP)](https://modelcontextprotocol.io).
It sits between any MCP client (Claude Desktop, Cursor, …) and any MCP server, and adds:

- **Transparent proxy** — stdio→stdio passthrough; the client is none the wiser.
- **Audit logging** — who / when / which tool / what params / what result, stored in SQLite (default) or PostgreSQL.
- **Parameter masking** — regex + field-name rules blank out PII / secrets before they hit the log.
- **Rate limiting** — per `client_id` token bucket (QPS) + daily quota.
- **Admin REST API** — query logs & stats for the Vue3 dashboard.

> Neutral, cross-client, single-binary. Cloud vendors optimize for their own
> ecosystem; MCPGW is the independent middle layer.

**Source:** https://github.com/dodoyu-sama/MCPGW

## Why MCPGW

A large, all-in-one MCP gateway is a fight you can't win against cloud vendors —
they optimize for their own ecosystems. The real gap is what they won't do finely:
**neutral governance of tool calls** that works across every MCP client.

MCPGW is a lightweight sidecar/proxy that sits between any MCP client (Claude
Desktop, Cursor, or domestic clients) and any MCP server, adding three things cloud
vendors leave to others:

- **Parameter masking** — auto-detect and redact PII / secrets in tool arguments.
- **Call audit** — who / when / which tool / what params / what result.
- **Replay** — re-issue a recorded `tools/call` to the upstream, for debugging and compliance.

It stays deliberately **decoupled from MCP protocol internals**: it lives at the
transport layer (stdio / SSE) and only knows the `tools/call` method name and its
params shape. As the MCP spec evolves, the governance layer keeps working without
rewrites.

## Architecture

```
MCP client  ──stdio──▶  mcpgw  ──stdio──▶  MCP server
 (Claude/Cursor)      │  intercept        (node/python/…)
                      │   ├─ rate limit
                      │   ├─ mask params
                      │   └─ audit ─▶ SQLite / PostgreSQL
                      └─ admin REST API :8080 ─▶ Vue3 dashboard
```

## Quick start (30s)

Requires **Go 1.22+** and a C compiler (CGO) for the SQLite driver:

```bash
brew install go            # macOS
xcode-select --install     # provides clang for CGO (macOS)
```

Build and run against the bundled demo server:

```bash
make build          # builds the web console (web/dist) then compiles the binary
make test           # run the Go test suite
# or, manually:
#   cd web && npm install && npm run build && cd ..
#   go build -o mcpgw ./cmd/mcpgw

# pipe a couple of JSON-RPC messages through the proxy
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' \
  '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"echo","arguments":{"message":"my email is a@b.com and pwd secret123"}}}' \
  | ./mcpgw --upstream "node examples/echo-server/server.js"
```

You should see the upstream responses printed, and a `mcpgw.db` SQLite file created.
Inspect the audit log via the admin API:

```bash
curl -H "Authorization: Bearer change-me" localhost:8080/api/logs
curl -H "Authorization: Bearer change-me" localhost:8080/api/stats
```

The `tools/call` above contains an email and a `pwd` field — both are stored
**masked** in the audit log, while the real request still reached the upstream.

## Transports

MCPGW decouples how **clients** connect from how it reaches the **upstream**
server. Both sides support `stdio` and `sse` (HTTP Server-Sent Events, the MCP
remote transport).

| `transport.client` | `transport.upstream` | 场景 |
|---|---|---|
| `stdio` (默认) | `stdio` (默认) | 本地透明代理，client 直接 spawn MCPGW |
| `sse` | `stdio` | **网关模式**：远端/多客户端经 HTTP 连 MCPGW，MCPGW 前端一个本地 stdio server |
| `sse` | `sse` | 完全远程：MCPGW 在中间做审计/脱敏，两侧都是 HTTP |
| `stdio` | `sse` | 把本地 client 的调用转发到远程 SSE server |

SSE 网关配置示例：

```yaml
server:
  upstream: ["node", "examples/mcp-server-demo/server.js"]
transport:
  client: sse
  listen: ":8081"
  upstream: stdio
```

运行后 MCP 客户端连 `http://host:8081/sse`（GET 建立事件流，POST
`/messages?sessionId=...` 发送请求）。多客户端并发时，MCPGW 用网关内部 id
改写做会话路由，再把上游响应的 `id` 还原给原客户端，因此审计与响应都不会串号。

CLI 也可覆盖：`--client-transport sse --listen :8081 --upstream-transport sse --upstream-url https://host/mcp/sse`。

### 端到端测试 `upstream: sse`

`examples/mcp-server-sse/server.js` 是一个零依赖的最小 SSE MCP server（监听
`:18080`），可用于验证 `upstream: sse`：

```bash
# 终端 1：启动上游 SSE server
node examples/mcp-server-sse/server.js

# 终端 2：MCPGW 以 stdio 对接 client、sse 对接上游
go build -o mcpgw ./cmd/mcpgw
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' \
  '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"lookup_customer","arguments":{"email":"alice@example.com","national_id":"11010519491231002X"}}}' \
  | ./mcpgw --config config.sse-upstream.yaml
```

## Wiring into a real client

Point your MCP client at `mcpgw` instead of the real server. For Claude Desktop
(`claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "my-server-via-mcpgw": {
      "command": "/path/to/mcpgw",
      "args": ["--upstream", "node", "/path/to/your-server.js"]
    }
  }
}
```

## Configuration

See `config.yaml`. Key sections:

| key | meaning |
|---|---|
| `server.client_id` | label stored on every audit record (or `MCPGW_CLIENT_ID`) |
| `server.upstream` | optional command+args written in config instead of `--upstream` |
| `audit` | `enabled`, `driver: sqlite` (default) or `postgres`, `dsn` (SQLite path or Postgres URL) |
| `transport` | `client` (`stdio`/`sse`), `upstream` (`stdio`/`sse`), `listen` (SSE bind addr) |
| `masking` | `enabled` + `rules` (regex `patterns` and/or `fields`) |
| `rate_limit` | `enabled`, `qps`, `daily_quota` (per client_id) |
| `admin` | `enabled`, `port`, `token` (Bearer token for the dashboard) |

Masking rules ship in `config.yaml` — email, credit card, API key, CN ID, and
sensitive-field presets are included as examples and can be extended there.

## Web dashboard

The Vue3 console is **compiled into the binary** (`web/embed.go` → `//go:embed
all:dist`) and served by the admin HTTP server. After `make build`, just open
`http://localhost:8080` — no separate frontend process needed.

```bash
make build
./mcpgw --config config.dev.yaml
# open http://localhost:8080  → Dashboard / Call Logs / Rules
```

For live frontend development (HMR) without rebuilding the binary:

```bash
./mcpgw --config config.dev.yaml &   # gateway + admin API on :8080
cd web && npm run dev                 # http://localhost:5173 (proxies /api -> :8080)
```

## Replay

Every `tools/call` is recorded with its original (unmasked) request params and the
raw upstream response. Replay re-issues a recorded call to the upstream — useful for
debugging flaky tools and for compliance re-execution.

```bash
# 1) find a call id
curl -H "Authorization: Bearer change-me" localhost:8080/api/logs

# 2) replay it (returns the upstream's raw JSON-RPC response)
curl -X POST -H "Authorization: Bearer change-me" -H "Content-Type: application/json" \
     -d '{"call_id": 1}' localhost:8080/api/replay
```

Replay is transport-agnostic: it rides the same id-rewrite / response-correlation
machinery as a live call, so it works for stdio and SSE upstreams alike.

## Docker

```bash
# SQLite audit backend (default) on :8080
docker compose up --build

# PostgreSQL audit backend — starts a postgres service + gateway on :8081
docker compose --profile postgres up --build

# point MCPGW at your own upstream — edit the command in docker-compose.yml
```

The `postgres` profile provisions a `postgres:16-alpine` container (data persisted
in the `pgdata` volume) plus a `mcpgw-postgres` gateway that loads
`config.docker-postgres.yaml`. For a standalone Postgres setup, copy
`config.postgres.yaml` and set `audit.driver: postgres` with your DSN.

## Roadmap

- v0.1 (this repo): stdio↔stdio / SSE transports, audit (SQLite + PostgreSQL), masking, rate limit, dashboard, replay.
- v0.2: LLM-assisted masking, rule CRUD in UI, export.
- v1.0: multi-tenancy, policy engine.
