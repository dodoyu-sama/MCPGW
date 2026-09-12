# MCPGW（中文文档）

面向 [Model Context Protocol (MCP)](https://modelcontextprotocol.io) 的透明代理 / sidecar。
它插在任意 MCP 客户端（Claude Desktop、Cursor……）与任意 MCP 服务端之间，提供：

- **透明代理** —— stdio→stdio 直通转发，客户端无感知。
- **调用审计** —— 谁 / 何时 / 调了哪个 tool / 传了什么参数 / 返回了什么，默认存 SQLite，也可存 PostgreSQL。
- **参数脱敏** —— 基于正则 + 字段名规则，在写入日志前抹掉 PII / 密钥。
- **限流** —— 按 `client_id` 的令牌桶（QPS）+ 每日配额。
- **管理 REST API** —— 供 Vue3 控制台查询日志与统计。

> 中立、跨客户端、单文件二进制。云厂商只优化自家生态；MCPGW 是独立的中间层。

**项目地址：** https://github.com/dodoyu-sama/MCPGW

## 为什么需要 MCPGW

大而全的 MCP 网关，是场打不赢云厂商的仗——他们只优化自己的生态。真正的缝隙在于他们不愿做细的事：**中立、可跨所有 MCP 客户端的 tool 调用治理**。

MCPGW 是一个轻量 sidecar / 代理，插在任意 MCP 客户端（Claude Desktop、Cursor，或各类国产客户端）与任意 MCP 服务端之间，补齐云厂商留给别人的三件事：

- **参数脱敏** —— 自动识别并遮蔽 tool 参数里的 PII / 密钥。
- **调用审计** —— 谁 / 何时 / 调了哪个 tool / 传了什么参数 / 返回了什么。
- **回放** —— 把记录过的一次 `tools/call` 重新发往上游，用于调试与合规。

它刻意**与 MCP 协议内部解耦**：只活在传输层（stdio / SSE），只认 `tools/call` 这个方法名和它的参数形状。MCP 规范演进时，治理层无需跟着重写。

## 架构

```
MCP client  ──stdio──▶  mcpgw  ──stdio──▶  MCP server
 (Claude/Cursor)      │  拦截处理          (node/python/…)
                      │   ├─ 限流
                      │   ├─ 参数脱敏
                      │   └─ 审计 ─▶ SQLite / PostgreSQL
                      └─ 管理 REST API :8080 ─▶ Vue3 控制台
```

## 快速开始（30 秒）

需要 **Go 1.22+** 以及用于 SQLite 驱动的 C 编译器（CGO）：

```bash
brew install go            # macOS
xcode-select --install     # 提供 CGO 所需的 clang（macOS）
```

构建并运行内置的演示服务端：

```bash
make build          # 先构建 web 控制台（web/dist），再编译二进制
make test           # 运行 Go 测试套件
# 或手动：
#   cd web && npm install && npm run build && cd ..
#   go build -o mcpgw ./cmd/mcpgw

# 通过代理管道发几条 JSON-RPC 消息
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' \
  '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"echo","arguments":{"message":"my email is a@b.com and pwd secret123"}}}' \
  | ./mcpgw --upstream "node examples/echo-server/server.js"
```

你会看到上游的响应被打印出来，同时生成一个 `mcpgw.db` SQLite 文件。
通过管理 API 查看审计日志：

```bash
curl -H "Authorization: Bearer change-me" localhost:8080/api/logs
curl -H "Authorization: Bearer change-me" localhost:8080/api/stats
```

上面的 `tools/call` 包含一个邮箱和一个 `pwd` 字段——二者在审计日志里都以**脱敏**形式存储，而真实请求仍然原样到达了上游。

## 传输方式

MCPGW 把**客户端如何连接**与**如何到达上游**服务端解耦。两侧都支持 `stdio` 与 `sse`（HTTP Server-Sent Events，即 MCP 的远程传输方式）。

| `transport.client` | `transport.upstream` | 场景 |
|---|---|---|
| `stdio`（默认） | `stdio`（默认） | 本地透明代理，client 直接 spawn MCPGW |
| `sse` | `stdio` | **网关模式**：远端 / 多客户端经 HTTP 连 MCPGW，MCPGW 前端一个本地 stdio server |
| `sse` | `sse` | 完全远程：MCPGW 在中间做审计 / 脱敏，两侧都是 HTTP |
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

## 接入真实客户端

把你的 MCP 客户端指向 `mcpgw`，而非真实服务端。以 Claude Desktop
（`claude_desktop_config.json`）为例：

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

## 配置

详见 `config.yaml`。关键配置项：

| key | 含义 |
|---|---|
| `server.client_id` | 写入每条审计记录的标签（或环境变量 `MCPGW_CLIENT_ID`） |
| `server.upstream` | 写在配置里、替代 `--upstream` 的命令 + 参数 |
| `audit` | `enabled`、`driver: sqlite`（默认）或 `postgres`、`dsn`（SQLite 路径或 Postgres 连接串） |
| `transport` | `client`（`stdio`/`sse`）、`upstream`（`stdio`/`sse`）、`listen`（SSE 监听地址） |
| `masking` | `enabled` + `rules`（正则 `patterns` 和 / 或 `fields`） |
| `rate_limit` | `enabled`、`qps`、`daily_quota`（按 client_id） |
| `admin` | `enabled`、`port`、`token`（控制台的 Bearer Token） |

脱敏规则随 `config.yaml` 一同提供——邮箱、信用卡、API Key、身份证，以及敏感字段预设都已作为示例内置，可在该文件里扩展。

## Web 控制台

Vue3 控制台被**编译进二进制**（`web/embed.go` → `//go:embed all:dist`），由管理 HTTP
服务直接托管。`make build` 之后直接打开 `http://localhost:8080` 即可，无需单独的前端进程。

```bash
make build
./mcpgw --config config.dev.yaml
# 打开 http://localhost:8080  → 仪表盘 / 调用日志 / 规则
```

如需在不重新编译二进制的情况下做前端热更新开发：

```bash
./mcpgw --config config.dev.yaml &   # 网关 + 管理 API 在 :8080
cd web && npm run dev                 # http://localhost:5173（/api 代理到 :8080）
```

## 回放

每一次 `tools/call` 都会记录其原始（未脱敏）请求参数与上游原始响应。回放把记录过的一次调用重新发往上游——可用于调试不稳定的 tool，以及合规性的重新执行。

```bash
# 1) 找到调用 id
curl -H "Authorization: Bearer change-me" localhost:8080/api/logs

# 2) 回放（返回上游的原始 JSON-RPC 响应）
curl -X POST -H "Authorization: Bearer change-me" -H "Content-Type: application/json" \
     -d '{"call_id": 1}' localhost:8080/api/replay
```

回放与传输方式无关：它复用和实时调用相同的 id 改写 / 响应关联机制，因此 stdio 与 SSE 上游都适用。

## Docker

```bash
# SQLite 审计后端（默认）在 :8080
docker compose up --build

# PostgreSQL 审计后端 —— 启动一个 postgres 服务 + 网关在 :8081
docker compose --profile postgres up --build

# 让 MCPGW 指向你自己的上游 —— 修改 docker-compose.yml 中的 command
```

`postgres` 配置档会拉起一个 `postgres:16-alpine` 容器（数据持久化在 `pgdata` 卷），
外加一个加载 `config.docker-postgres.yaml` 的 `mcpgw-postgres` 网关。若要独立部署
Postgres，复制 `config.postgres.yaml` 并将 `audit.driver` 设为 `postgres` 并填好你的 DSN。

## 路线图

- v0.1（本仓库）：stdio↔stdio / SSE 传输、审计（SQLite + PostgreSQL）、脱敏、限流、控制台、回放。
- v0.2：LLM 辅助脱敏、UI 内规则增删改、导出。
- v1.0：多租户、策略引擎。
