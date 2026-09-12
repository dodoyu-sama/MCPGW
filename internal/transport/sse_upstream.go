package transport

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// SSEUpstream connects to a remote MCP server that speaks the SSE transport.
// It opens GET <url> to receive messages, learns the POST endpoint from the
// `endpoint` event, and sends client requests via POST.
type SSEUpstream struct {
	url     string
	client  *http.Client
	mu      sync.Mutex
	postURL string
}

func NewSSEUpstream(rawURL string) *SSEUpstream {
	return &SSEUpstream{url: rawURL, client: &http.Client{}}
}

func (u *SSEUpstream) Run(ctx context.Context, onMessage func([]byte)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.url, nil)
	if err != nil {
		return err
	}
	resp, err := u.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)
	var event, data string
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			switch event {
			case "endpoint":
				u.setPostURL(data)
			case "message":
				cp := make([]byte, len(data))
				copy(cp, data)
				onMessage(cp)
			}
			event, data = "", ""
			continue
		}
		switch {
		case strings.HasPrefix(line, "event:"):
			event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			data = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		}
	}
	return scanner.Err()
}

func (u *SSEUpstream) setPostURL(data string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if strings.HasPrefix(data, "http://") || strings.HasPrefix(data, "https://") {
		u.postURL = data
		return
	}
	// relative path: resolve against the SSE endpoint origin
	base, err := url.Parse(u.url)
	if err != nil {
		u.postURL = data
		return
	}
	u.postURL = base.Scheme + "://" + base.Host + data
}

// Write forwards a client request to the upstream messages endpoint, waiting
// briefly for the `endpoint` event (post URL) to be established first.
func (u *SSEUpstream) Write(raw []byte) error {
	postURL, err := u.waitPostURL(5 * time.Second)
	if err != nil {
		return err
	}
	resp, err := u.client.Post(postURL, "application/json", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	return nil
}

func (u *SSEUpstream) waitPostURL(timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for {
		u.mu.Lock()
		postURL := u.postURL
		u.mu.Unlock()
		if postURL != "" {
			return postURL, nil
		}
		if time.Now().After(deadline) {
			return "", errors.New("sse upstream not ready: no messages endpoint")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func (u *SSEUpstream) Close() error {
	u.client.CloseIdleConnections()
	return nil
}
