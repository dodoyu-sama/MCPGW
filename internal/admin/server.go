package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strings"

	"github.com/dodoyu-sama/mcp-arc/internal/audit"
	"github.com/dodoyu-sama/mcp-arc/web"
)

// Replayer re-issues a previously recorded tools/call to the upstream server.
// It is satisfied by *proxy.Proxy, keeping admin decoupled from the proxy package.
type Replayer interface {
	Replay(ctx context.Context, toolName string, rawParams []byte) ([]byte, error)
}

type Server struct {
	store    audit.Store
	token    string
	replayer Replayer
	rules    RuleManager
}

func New(store audit.Store, token string, replayer Replayer, rules RuleManager) *Server {
	return &Server{store: store, token: token, replayer: replayer, rules: rules}
}

func (s *Server) Start(port int) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/logs", s.auth(s.handleLogs))
	mux.HandleFunc("/api/stats", s.auth(s.handleStats))
	mux.HandleFunc("/api/replay", s.auth(s.handleReplay))
	mux.HandleFunc("/api/export", s.auth(s.handleExport))
	mux.HandleFunc("/api/rules", s.auth(s.handleRules))
	mux.HandleFunc("/api/rules/{id}", s.auth(s.handleRuleByID))

	// Serve the embedded web console (compiled into the binary by `npm run build`).
	sub, err := fs.Sub(web.DistFS, "dist")
	if err != nil {
		return fmt.Errorf("embed web dist: %w", err)
	}
	fileServer := http.FileServer(http.FS(sub))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// SPA fallback: unknown non-asset routes return index.html.
		if r.URL.Path != "/" && !strings.HasPrefix(r.URL.Path, "/api/") {
			if _, statErr := fs.Stat(sub, strings.TrimPrefix(r.URL.Path, "/")); statErr != nil {
				r.URL.Path = "/"
			}
		}
		fileServer.ServeHTTP(w, r)
	})

	return http.ListenAndServe(fmt.Sprintf(":%d", port), mux)
}

func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.token != "" {
			if r.Header.Get("Authorization") != "Bearer "+s.token {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		next(w, r)
	}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
