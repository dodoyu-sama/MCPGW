// A more realistic MCP stdio server (no SDK) to exercise MCP Arc end-to-end.
// It exposes tools that take PII / secrets so we can verify masking + audit.
const readline = require('readline')

const rl = readline.createInterface({ input: process.stdin, terminal: false })

function send(obj) {
  process.stdout.write(JSON.stringify(obj) + '\n')
}

const TOOLS = [
  {
    name: 'lookup_customer',
    description: 'Look up a customer by email / national ID.',
    inputSchema: {
      type: 'object',
      properties: {
        email: { type: 'string' },
        national_id: { type: 'string' },
        name: { type: 'string' },
      },
      required: ['email'],
    },
  },
  {
    name: 'charge_card',
    description: 'Charge a credit card.',
    inputSchema: {
      type: 'object',
      properties: {
        card_number: { type: 'string' },
        cvv: { type: 'string' },
        amount: { type: 'number' },
      },
      required: ['card_number', 'amount'],
    },
  },
  {
    name: 'send_email',
    description: 'Send an email via a provider API.',
    inputSchema: {
      type: 'object',
      properties: {
        to: { type: 'string' },
        subject: { type: 'string' },
        body: { type: 'string' },
        api_key: { type: 'string' },
      },
      required: ['to', 'api_key'],
    },
  },
]

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
        serverInfo: { name: 'mcp-demo', version: '0.1.0' },
      },
    })
  } else if (method === 'notifications/initialized') {
    // no response
  } else if (method === 'tools/list') {
    send({ jsonrpc: '2.0', id, result: { tools: TOOLS } })
  } else if (method === 'tools/call') {
    const name = params && params.name
    const a = (params && params.arguments) || {}
    // Echo back what the server actually received, so we can prove the REAL
    // (unmasked) data reached the upstream.
    let result
    if (name === 'charge_card') {
      const digits = String(a.card_number || '').replace(/\D/g, '')
      result = { charged: a.amount, card_last4: digits.slice(-4), cvv_ok: !!a.cvv }
    } else if (name === 'lookup_customer') {
      result = { found: true, email: a.email, national_id: a.national_id, name: a.name }
    } else if (name === 'send_email') {
      result = { sent: true, to: a.to, used_key: String(a.api_key || '').slice(0, 6) + '…' }
    } else {
      result = { echo: a }
    }
    send({
      jsonrpc: '2.0', id,
      result: { content: [{ type: 'text', text: JSON.stringify(result) }], isError: false },
    })
  } else {
    send({ jsonrpc: '2.0', id, error: { code: -32601, message: 'method not found: ' + method } })
  }
})
