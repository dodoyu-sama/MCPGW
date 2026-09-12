// Minimal MCP stdio server (no SDK needed) used to smoke-test MCPGW.
// It implements initialize / notifications/initialized / tools/list / tools/call.
const readline = require('readline')

const rl = readline.createInterface({ input: process.stdin, terminal: false })

function send(obj) {
  process.stdout.write(JSON.stringify(obj) + '\n')
}

const TOOLS = [{
  name: 'echo',
  description: 'Echo back the arguments (demo tool for MCPGW)',
  inputSchema: {
    type: 'object',
    properties: { message: { type: 'string' } },
    required: ['message'],
  },
}]

rl.on('line', (line) => {
  let msg
  try { msg = JSON.parse(line) } catch { return }
  const { id, method, params } = msg || {}
  if (method === 'initialize') {
    send({
      jsonrpc: '2.0', id,
      result: {
        protocolVersion: '2024-11-05',
        capabilities: { tools: {} },
        serverInfo: { name: 'echo-demo', version: '0.1.0' },
      },
    })
  } else if (method === 'notifications/initialized') {
    // no response for notifications
  } else if (method === 'tools/list') {
    send({ jsonrpc: '2.0', id, result: { tools: TOOLS } })
  } else if (method === 'tools/call') {
    const name = params && params.name
    const args = (params && params.arguments) || {}
    send({
      jsonrpc: '2.0', id,
      result: {
        content: [{ type: 'text', text: JSON.stringify({ tool: name, echo: args }) }],
        isError: false,
      },
    })
  } else {
    send({ jsonrpc: '2.0', id, error: { code: -32601, message: 'method not found: ' + method } })
  }
})
