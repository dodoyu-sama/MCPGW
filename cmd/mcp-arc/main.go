package main

import (
	"flag"
	"log"
	"strings"

	"github.com/dodoyu-sama/mcp-arc/internal/config"
	"github.com/dodoyu-sama/mcp-arc/internal/proxy"
)

func main() {
	var (
		configPath      string
		upstream        string
		clientTransport string
		listen          string
		upstreamTrans   string
		upstreamURL     string
	)
	flag.StringVar(&configPath, "config", "config.yaml", "path to YAML config file")
	flag.StringVar(&upstream, "upstream", "", "upstream command, e.g. 'node server.js'")
	flag.StringVar(&clientTransport, "client-transport", "", "client transport: stdio | sse")
	flag.StringVar(&listen, "listen", "", "listen address for SSE client transport, e.g. :8081")
	flag.StringVar(&upstreamTrans, "upstream-transport", "", "upstream transport: stdio | sse")
	flag.StringVar(&upstreamURL, "upstream-url", "", "upstream /sse URL when upstream transport = sse")
	flag.Parse()

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Printf("warn: could not load config (%v), using defaults", err)
		cfg = config.Default()
	}

	// CLI overrides
	if clientTransport != "" {
		cfg.Transport.Client = clientTransport
	}
	if listen != "" {
		cfg.Transport.Listen = listen
	}
	if upstreamTrans != "" {
		cfg.Transport.Upstream = upstreamTrans
	}
	if upstreamURL != "" {
		cfg.Transport.UpstreamURL = upstreamURL
	}

	var upstreamCmd []string
	switch {
	case upstream != "":
		upstreamCmd = strings.Fields(upstream)
	case len(cfg.Server.Upstream) > 0:
		upstreamCmd = cfg.Server.Upstream
	default:
		upstreamCmd = flag.Args()
	}

	p := proxy.New(proxy.Options{
		UpstreamCmd: upstreamCmd,
		Config:      cfg,
	})
	if err := p.Run(); err != nil {
		log.Fatal(err)
	}
}
