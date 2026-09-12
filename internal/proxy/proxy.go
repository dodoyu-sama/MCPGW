// Package proxy implements an MCP-agnostic relay that sits between an MCP client
// and an MCP server. It deliberately lives at the transport layer: the only
// MCP-specific knowledge it relies on is the "tools/call" JSON-RPC method name
// and its params shape. It does NOT parse or depend on MCP protocol internals, so
// the governance features (masking, audit, rate limit, replay) keep working even
// as the MCP spec evolves.
package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dodoyu-sama/mcpgw/internal/admin"
	"github.com/dodoyu-sama/mcpgw/internal/audit"
	"github.com/dodoyu-sama/mcpgw/internal/config"
	"github.com/dodoyu-sama/mcpgw/internal/mask"
	"github.com/dodoyu-sama/mcpgw/internal/ratelimit"
	"github.com/dodoyu-sama/mcpgw/internal/transport"
)

type Options struct {
	UpstreamCmd []string
	Config      *config.Config
}

type Proxy struct {
	opts       Options
	auditStore audit.Store
	masker     *mask.Masker
	limiter    *ratelimit.TokenBucketManager
	clientID   string

	upstreamWriter  func([]byte) error
	clientBroadcast func([]byte) error

	mu      sync.Mutex
	seq     int64
	pending map[string]*pendingCall
}

func New(opts Options) *Proxy {
	p := &Proxy{
		opts:     opts,
		clientID: opts.Config.Server.ClientID,
		pending:  make(map[string]*pendingCall),
	}

	if opts.Config.Audit.Enabled {
		store, err := audit.NewStore(opts.Config.Audit.Driver, opts.Config.Audit.DSN)
		if err != nil {
			log.Printf("warn: audit store init failed: %v", err)
		} else {
			p.auditStore = store
		}
	}

	if opts.Config.Masking.Enabled {
		m, err := mask.New(opts.Config.Masking.Rules)
		if err != nil {
			log.Printf("warn: masker init failed: %v", err)
		} else {
			p.masker = m
		}
	}

	p.limiter = ratelimit.NewTokenBucketManager(
		opts.Config.RateLimit.QPS,
		opts.Config.RateLimit.DailyQuota,
		opts.Config.RateLimit.Enabled,
	)
	return p
}

// Run wires up the client and upstream transports and pumps messages between
// them, applying the interception chain (rate limit, mask, audit) on the way.
func (p *Proxy) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// --- upstream transport ---
	var upstream transport.UpstreamTransporter
	var err error
	switch p.opts.Config.Transport.Upstream {
	case "sse":
		if p.opts.Config.Transport.UpstreamURL == "" {
			return errors.New("transport.upstream_url is required when upstream = sse")
		}
		upstream = transport.NewSSEUpstream(p.opts.Config.Transport.UpstreamURL)
	default: // stdio
		if len(p.opts.UpstreamCmd) == 0 {
			return errors.New("no upstream command provided")
		}
		upstream, err = transport.NewStdioUpstream(p.opts.UpstreamCmd)
		if err != nil {
			return err
		}
	}
	p.upstreamWriter = upstream.Write

	// --- client transport ---
	var client transport.ClientTransporter
	switch p.opts.Config.Transport.Client {
	case "sse":
		client = transport.NewSSEServer(p.opts.Config.Transport.Listen)
		log.Printf("mcpgw: SSE client transport listening on %s", p.opts.Config.Transport.Listen)
	default: // stdio
		client = transport.StdioClient{}
	}

	// broadcast target for upstream-initiated notifications
	if sse, ok := client.(*transport.SSEServer); ok {
		p.clientBroadcast = sse.Broadcast
	} else {
		p.clientBroadcast = func(b []byte) error {
			os.Stdout.Write(b)
			_, e := os.Stdout.Write([]byte{'\n'})
			return e
		}
	}

	if p.opts.Config.Admin.Enabled && p.auditStore != nil {
		go func() {
			srv := admin.New(p.auditStore, p.opts.Config.Admin.Token, p)
			if e := srv.Start(p.opts.Config.Admin.Port); e != nil {
				log.Printf("warn: admin server stopped: %v", e)
			}
		}()
	}

	errCh := make(chan error, 2)
	go func() {
		if e := upstream.Run(ctx, func(raw []byte) {
			if err := p.processUpstreamMessage(raw); err != nil {
				if errors.Is(err, context.Canceled) {
					return // client disconnected; nothing to deliver, not an error
				}
				log.Printf("warn: upstream message: %v", err)
			}
		}); e != nil {
			errCh <- e
		} else {
			errCh <- fmt.Errorf("upstream closed")
		}
	}()
	go func() {
		if e := client.Run(ctx, func(raw []byte, respond func([]byte) error) {
			p.handleClientMessage(raw, respond)
		}); e != nil {
			errCh <- e
		} else {
			errCh <- fmt.Errorf("client closed")
		}
	}()

	log.Printf("mcpgw: running (client=%s, upstream=%s)", p.opts.Config.Transport.Client, p.opts.Config.Transport.Upstream)
	err = <-errCh
	log.Printf("mcpgw: shutting down (%v)", err)
	cancel()
	_ = upstream.Close()
	_ = client.Close()
	if p.auditStore != nil {
		_ = p.auditStore.Close()
	}
	return nil
}

func (p *Proxy) handleClientMessage(raw []byte, respond func([]byte) error) {
	forward, synthetic := p.processClientMessage(raw, respond)
	if synthetic != nil {
		_ = respond(synthetic)
	} else if forward != nil {
		_ = p.upstreamWriter(forward)
	}
}

// Replay re-issues a previously recorded tools/call to the upstream server using
// the original (unmasked) request parameters, and returns the upstream's raw
// response. It rides the same pending/id-rewrite machinery as a live call so the
// response is correlated and audited normally. This is intentionally decoupled
// from MCP semantics: it only knows the "tools/call" method name and the params
// shape — it does not parse or depend on the protocol internals.
func (p *Proxy) Replay(ctx context.Context, toolName string, rawParams []byte) ([]byte, error) {
	if p.upstreamWriter == nil {
		return nil, errors.New("replay unavailable: upstream transport not connected")
	}
	var args map[string]interface{}
	if err := json.Unmarshal(rawParams, &args); err != nil || args == nil {
		args = map[string]interface{}{}
	}
	upID := fmt.Sprintf("gw-%d", atomic.AddInt64(&p.seq, 1))
	ch := make(chan []byte, 1)
	pc := &pendingCall{
		toolName: toolName,
		start:    time.Now(),
		respond:  func(b []byte) error { ch <- b; return nil },
		origID:   upID,
	}
	if p.masker != nil {
		if m, err := p.masker.Mask(args); err == nil && m != nil {
			if mb, err := json.Marshal(m); err == nil {
				pc.maskedParams = string(mb)
			}
		}
	}
	pc.rawParams = string(rawParams)
	p.mu.Lock()
	p.pending[upID] = pc
	p.mu.Unlock()

	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      upID,
		"method":  "tools/call",
		"params":  map[string]interface{}{"name": toolName, "arguments": args},
	}
	b, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	if err := p.upstreamWriter(b); err != nil {
		return nil, err
	}
	select {
	case resp := <-ch:
		return resp, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(30 * time.Second):
		return nil, errors.New("replay timeout")
	}
}
