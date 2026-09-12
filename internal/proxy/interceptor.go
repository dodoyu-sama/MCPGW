package proxy

import (
	"encoding/json"
	"fmt"
	"log"
	"sync/atomic"
	"time"

	"github.com/dodoyu-sama/mcpgw/internal/audit"
)

type pendingCall struct {
	toolName     string
	maskedParams string
	rawParams    string
	start        time.Time
	respond      func([]byte) error
	origID       interface{}
}

// processClientMessage intercepts a message from an MCP client. It applies rate
// limiting and (for tools/call) masking + audit prep, rewrites the JSON-RPC id
// to a gateway-unique value for correlation/routing across concurrent sessions,
// and returns the bytes to forward upstream. On rate limiting it returns a
// synthetic error to send back to the client instead.
func (p *Proxy) processClientMessage(raw []byte, respond func([]byte) error) (forward []byte, synthetic []byte) {
	var msg map[string]interface{}
	if err := json.Unmarshal(raw, &msg); err != nil {
		return raw, nil // not JSON-RPC, pass through untouched
	}
	id, hasID := msg["id"]
	method, _ := msg["method"].(string)
	if !hasID {
		// notification: no response expected, pass through
		return raw, nil
	}

	// rate limit only tool calls
	if method == "tools/call" && !p.limiter.Allow(p.clientID) {
		resp := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      id,
			"error":   map[string]interface{}{"code": -32000, "message": "rate limit exceeded"},
		}
		b, _ := json.Marshal(resp)
		return nil, b
	}

	// gateway-unique id for correlation across concurrent client sessions
	upID := fmt.Sprintf("gw-%d", atomic.AddInt64(&p.seq, 1))
	pc := &pendingCall{respond: respond, origID: id, start: time.Now()}

	if method == "tools/call" {
		params, _ := msg["params"].(map[string]interface{})
		toolName, _ := params["name"].(string)
		arguments, _ := params["arguments"].(map[string]interface{})
		if arguments == nil {
			arguments = map[string]interface{}{}
		}
		pc.toolName = toolName
		rawParams, _ := json.Marshal(arguments)
		maskedParams := rawParams
		if p.masker != nil {
			if m, err := p.masker.Mask(arguments); err == nil && m != nil {
				maskedParams, _ = json.Marshal(m)
			}
		}
		pc.rawParams = string(rawParams)
		pc.maskedParams = string(maskedParams)
	}

	p.mu.Lock()
	p.pending[upID] = pc
	p.mu.Unlock()

	msg["id"] = upID
	out, _ := json.Marshal(msg)
	return out, nil
}

// processUpstreamMessage intercepts a response from the upstream server, writes
// an audit record for tools/call responses, restores the original client id, and
// routes the message back to the correct client session.
func (p *Proxy) processUpstreamMessage(raw []byte) error {
	var msg map[string]interface{}
	if err := json.Unmarshal(raw, &msg); err != nil {
		return nil
	}
	id, ok := msg["id"]
	if !ok {
		// upstream-initiated notification: broadcast to all clients
		return p.clientBroadcast(raw)
	}
	key := fmt.Sprintf("%v", id)

	p.mu.Lock()
	pc, ok := p.pending[key]
	if !ok {
		p.mu.Unlock()
		return nil
	}
	delete(p.pending, key)
	p.mu.Unlock()

	// restore the original client id
	msg["id"] = pc.origID
	outRaw, _ := json.Marshal(msg)

	if pc.toolName != "" {
		latency := time.Since(pc.start).Milliseconds()
		errMsg := ""
		var result interface{}
		if e, ok := msg["error"].(map[string]interface{}); ok {
			b, _ := json.Marshal(e)
			errMsg = string(b)
		} else {
			result = msg["result"]
		}
		rawResultBytes := []byte("null")
		if result != nil {
			rawResultBytes, _ = json.Marshal(result)
		}
		resultBytes := []byte("null")
		if result != nil {
			if p.masker != nil {
				if rm, ok := result.(map[string]interface{}); ok {
					if masked, err := p.masker.Mask(rm); err == nil && masked != nil {
						result = masked
					}
				}
			}
			resultBytes, _ = json.Marshal(result)
		}
		rec := &audit.CallRecord{
			ClientID:  p.clientID,
			ToolName:  pc.toolName,
			Params:    pc.maskedParams,
			RawParams: pc.rawParams,
			Result:    string(resultBytes),
			RawResult: string(rawResultBytes),
			ErrorMsg:  errMsg,
			LatencyMs: latency,
			Timestamp: time.Now(),
		}
		if p.auditStore != nil {
			if err := p.auditStore.Insert(rec); err != nil {
				log.Printf("warn: audit insert failed: %v", err)
			}
		}
	}

	return pc.respond(outRaw)
}
