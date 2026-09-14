package transport

import "context"

// ClientTransporter is how MCP Arc communicates with an MCP client.
// Run reads messages from the client and invokes onMessage for each JSON-RPC
// payload, together with a `respond` function that writes a message back to
// that specific client (used for synthetic errors and for routing responses).
type ClientTransporter interface {
	Run(ctx context.Context, onMessage func(raw []byte, respond func([]byte) error)) error
	// Broadcast writes a message to all connected clients (used for
	// upstream-initiated notifications that cannot be routed to one session).
	Broadcast(raw []byte) error
	Close() error
}

// UpstreamTransporter is how MCP Arc communicates with the upstream MCP server.
type UpstreamTransporter interface {
	Run(ctx context.Context, onMessage func(raw []byte)) error
	Write(raw []byte) error
	Close() error
}
