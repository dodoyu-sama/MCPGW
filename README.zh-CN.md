[English](./README.md) · [中文](./README.zh-CN.md)

# MCP Arc

一个**轻量的 MCP Proxy**，位于 MCP client 与 MCP server 之间。
一行命令，为任意 MCP 调用加上审计日志、参数脱敏和调用回放——不侵入协议，不绑定厂商。

```
MCP client  ──────▶  MCP Arc  ──────▶  MCP server
(Claude / Cursor /    │  治理层           (node / python / …)
 国产客户端 …)        │
                      └─ 审计 ─▶ SQLite / PostgreSQL
```

**项目地址：** https://github.com/dodoyu-sama/MCP-Arc

## 核心功能

| | |
|---|---|
| **参数脱敏** | 自动识别并遮蔽 `tools/call` 参数（与返回）里的 PII / 密钥，再落库。正则 + 敏感字段名，YAML 里配，或运行时改。 |
| **调用审计** | 谁（`client_id`）、何时、调了哪个 tool、传了什么参数、返回了什么、耗时多少、成功还是失败——落 SQLite（默认）或 PostgreSQL。 |
| **回放** | 把记录过的一次 `tools/call` 原样重新发往上游，用于调试不稳定的 tool，以及合规性的重新执行。 |

外加让它真正能用的小事：按 `client_id` 的限流（令牌桶 QPS + 日配额）、运行时可改的
脱敏规则（改完即生效，无需重启）、JSON / CSV 导出，以及编译进单个二进制的 Vue3 控制台。

## 设计

- 工作在**传输层**（stdio / SSE）之上，不深入 protocol 内部。
- 唯一依赖的 MCP 知识，是 `tools/call` 这个方法名和 `params.{name,arguments}` 的参数形状。
- 请求进来时把 JSON-RPC 的 `id` 改写成内部 id，响应回来时还原，以此做关联与路由——
  响应是**匹配**回来的，不是**解析**出来的。其余部分都是与协议无关的管道，spec 演进
  不需要重写治理层。
- 不需要处理的消息，逐字节原样转发。

## 快速开始

需要 **Go 1.22+** 以及用于 SQLite 驱动的 C 编译器（CGO）。

```bash
make build     # 先构建 web 控制台（web/dist），再编译 ./mcp-arc
make test
```

通过代理管道发几条 JSON-RPC 消息：

```bash
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' \
  '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"echo","arguments":{"message":"my email is a@b.com and pwd secret123"}}}' \
  | ./mcp-arc --upstream "node examples/echo-server/server.js"
```

你会看到上游的响应被打印出来，同时生成 `mcp-arc.db`。上面的邮箱和 `pwd` 字段在审计
日志里以**脱敏**形式存储，而真实请求仍然原样到达了上游：

```bash
curl -H "Authorization: Bearer change-me" localhost:8080/api/logs
curl -H "Authorization: Bearer change-me" localhost:8080/api/stats
```

## 传输方式

客户端侧与上游侧独立配置，都支持 `stdio` 与 `sse`。

| `transport.client` | `transport.upstream` | 场景 |
|---|---|---|
| `stdio`（默认） | `stdio`（默认） | 本地透明代理，client 直接 spawn MCP Arc |
| `sse` | `stdio` | **网关模式**：远端 / 多客户端经 HTTP 连 MCP Arc，Arc 前端一个本地 stdio server |
| `sse` | `sse` | 完全远程：Arc 在中间做治理，两侧都是 HTTP |
| `stdio` | `sse` | 把本地 client 的调用转发到远程 SSE server |

```yaml
server:
  upstream: ["node", "examples/mcp-server-demo/server.js"]
transport:
  client: sse
  listen: ":8081"
  upstream: stdio
```

运行后 MCP 客户端连 `http://host:8081/sse`（GET 建立事件流，POST
`/messages?sessionId=...` 发送请求）。

### 命令行参数

`--config`、`--upstream`、`--client-transport`（`stdio`|`sse`）、`--listen`、
`--upstream-transport`（`stdio`|`sse`）、`--upstream-url`。

```bash
./mcp-arc --client-transport sse --listen :8081 \
          --upstream-transport sse --upstream-url https://host/mcp/sse
```

## 接入真实客户端

把 MCP 客户端指向 `mcp-arc`，而非真实服务端：

```json
{
  "mcpServers": {
    "my-server-via-mcp-arc": {
      "command": "/path/to/mcp-arc",
      "args": ["--upstream", "node", "/path/to/your-server.js"]
    }
  }
}
```

如果客户端支持以 SSE 方式接入远端 MCP，就让 Arc 跑在网关模式
（`transport.client: sse`），把 `http://host:8081/sse` 填进去即可。

## 配置

详见 `config.yaml`。关键配置项：

| key | 含义 |
|---|---|
| `server.client_id` | 写入每条审计记录的标签（或环境变量 `MCP_ARC_CLIENT_ID`） |
| `server.upstream` | 写在配置里、替代 `--upstream` 的命令 + 参数 |
| `transport` | `client` / `upstream`（`stdio`\|`sse`）、`listen`（SSE 监听地址）、`upstream_url` |
| `audit` | `enabled`、`driver: sqlite`（默认）或 `postgres`、`dsn` |
| `masking` | `enabled` + `rules`（正则 `patterns` 和 / 或 `fields`） |
| `llm` | 可选的 LLM 辅助脱敏：`enabled`、`endpoint`、`api_key`、`model`、`timeout_ms`、`max_bytes`、`cache_ttl_seconds`、`apply_to_result` |
| `rate_limit` | `enabled`、`qps`、`daily_quota`（按 client_id） |
| `admin` | `enabled`、`port`、`token`（控制台的 Bearer Token） |

## 脱敏规则

规则存在库里（`mask_rules` 表），不再只活在 YAML 中：首次启动时 `config.yaml`
里的规则会被**种子化**写入表中（`source: config`），之后规则归控制台管——可新建、
编辑、启停、删除，**每次写入都会热加载**，下一条 tool 调用就用新规则，不用重启。

一条规则必须有 `name`，且至少有 `pattern` 或 `field` 之一；正则在保存时会做编译校验，
写错的正则会被当场拒绝，而不是悄悄让脱敏失效。

```bash
curl -H "Authorization: Bearer change-me" localhost:8080/api/rules

curl -X POST -H "Authorization: Bearer change-me" -H "Content-Type: application/json" \
  -d '{"name":"phone_cn","patterns":["\\b1[3-9]\\d{9}\\b"],"mask_char":"[PHONE]"}' \
  localhost:8080/api/rules
```

## LLM 辅助脱敏

可选的**第二遍**，补静态规则抓不到的自由文本 PII。模型只拿到**已脱敏**的 payload，
只被问"还有哪些路径仍然敏感"；已被抹掉的值不会离开进程，幻觉出来的路径直接忽略。
全程 **fail-open**：出错、超时、或 payload 超过 `max_bytes`，就沿用静态脱敏的结果，
调用照常继续。

```yaml
llm:
  enabled: true
  endpoint: "https://api.openai.com/v1"   # 任意 OpenAI 兼容端点
  # api_key 建议用环境变量 MCP_ARC_LLM_API_KEY
  model: "gpt-4o-mini"
  timeout_ms: 3000
  max_bytes: 8192
  apply_to_result: false                  # 是否也扫上游返回
```

## 导出

```bash
curl -H "Authorization: Bearer change-me" "localhost:8080/api/export?format=csv"
curl -H "Authorization: Bearer change-me" "localhost:8080/api/export?format=json&limit=5000"
curl -H "Authorization: Bearer change-me" "localhost:8080/api/export?format=json&raw=1"  # 含未脱敏值
```

支持按 `client_id` / `tool` / `limit` 过滤（`limit` 上限 10000）。导出内容是**已脱敏**
的参数与结果；未脱敏的 `raw_params` / `raw_result` 只有显式带 `raw=1` 才会带上。
控制台「调用日志」页的 Export JSON / CSV 按钮走的是同一个接口。

## Web 控制台

Vue3 控制台被**编译进二进制**（`//go:embed`），由管理 HTTP 服务直接托管，无需单独的
前端进程。

```bash
./mcp-arc --config config.dev.yaml
# 打开 http://localhost:8080  →  仪表盘 / 调用日志 / 规则
```

## 回放

每一次 `tools/call` 都会记录其原始（未脱敏）请求参数与上游原始响应：

```bash
# 1) 找到调用 id
curl -H "Authorization: Bearer change-me" localhost:8080/api/logs

# 2) 回放（返回上游的原始 JSON-RPC 响应）
curl -X POST -H "Authorization: Bearer change-me" -H "Content-Type: application/json" \
     -d '{"call_id": 1}' localhost:8080/api/replay
```

回放复用和实时调用相同的 id 改写 / 响应关联机制，因此 stdio 与 SSE 上游都适用。

## Docker

```bash
docker compose up --build                     # SQLite 后端，控制台在 :8080
docker compose --profile postgres up --build  # PostgreSQL 后端
```

## 路线图

- **v0.1** ✅ stdio / SSE 传输、审计（SQLite + PostgreSQL）、脱敏、限流、控制台、回放。
- **v0.2** ✅ LLM 辅助脱敏、控制台内规则增删改、JSON / CSV 导出。
- **v0.3**（进行中）——稳定性与生产可用性：配置热加载、上游异常退出的优雅处理、审计写入失败降级、告警 webhook、统计面板增强、预置脱敏规则模板完善、7×24 稳定性测试。
- **v1.0** ——生产环境可用：文档完整、一键安装（脚本 / brew / `go install` / Docker）、SSE 模式下的 TLS。
- **v2.0+** ——多租户、策略引擎、分布式部署。不承诺时间线。

## License

MIT —— 见 [LICENSE](./LICENSE)。
