// Minimal MCP server speaking the SSE transport, for end-to-end testing of
// MCP Arc's `upstream: sse` mode. No external dependencies.
//
//   node examples/mcp-server-sse/server.js
//
// It listens on :18080, serves GET /sse (event stream) and POST /messages, and
// implements a few demo tools.
const http = require('http');

const TOOLS = {
  lookup_customer: (a) => ({ found: true, email: a.email, national_id: a.national_id }),
  charge_card: (a) => ({
    charged: a.amount,
    card_last4: String(a.card_number || '').replace(/\s|-/g, '').slice(-4),
    cvv_ok: true,
  }),
  send_email: (a) => ({ sent: true, to: a.to, used_key: a.api_key }),
};

function handle(msg) {
  const id = msg.id;
  switch (msg.method) {
    case 'initialize':
      return {
        jsonrpc: '2.0', id,
        result: { protocolVersion: '2024-11-05', capabilities: { tools: {} }, serverInfo: { name: 'mcp-sse-demo', version: '0.1.0' } },
      };
    case 'tools/list':
      return {
        jsonrpc: '2.0', id,
        result: { tools: Object.keys(TOOLS).map((n) => ({ name: n, description: n, inputSchema: { type: 'object', properties: {} } })) },
      };
    case 'tools/call': {
      const { name, arguments: args } = msg.params || {};
      if (!TOOLS[name]) return { jsonrpc: '2.0', id, error: { code: -32601, message: 'unknown tool: ' + name } };
      return { jsonrpc: '2.0', id, result: { content: [{ type: 'text', text: JSON.stringify(TOOLS[name](args || {})) }], isError: false } };
    }
    default:
      return { jsonrpc: '2.0', id, error: { code: -32601, message: 'method not found: ' + msg.method } };
  }
}

const sessions = new Map();

const server = http.createServer((req, res) => {
  if (req.method === 'GET' && req.url === '/sse') {
    res.writeHead(200, {
      'Content-Type': 'text/event-stream',
      'Cache-Control': 'no-cache',
      'Connection': 'keep-alive',
    });
    const sid = Math.random().toString(36).slice(2);
    sessions.set(sid, res);
    res.write(`event: endpoint\ndata: /messages?sessionId=${sid}\n\n`);
    req.on('close', () => sessions.delete(sid));
    return;
  }
  if (req.method === 'POST' && req.url.startsWith('/messages')) {
    const sid = new URL(req.url, 'http://localhost').searchParams.get('sessionId');
    const stream = sessions.get(sid);
    let body = '';
    req.on('data', (c) => (body += c));
    req.on('end', () => {
      res.writeHead(202);
      res.end();
      let msg;
      try {
        msg = JSON.parse(body);
      } catch (e) {
        return;
      }
      if (stream && msg.id !== undefined) {
        stream.write(`event: message\ndata: ${JSON.stringify(handle(msg))}\n\n`);
      }
    });
    return;
  }
  res.writeHead(404);
  res.end();
});

server.listen(18080, () => console.error('mcp-sse-demo listening on :18080'));
