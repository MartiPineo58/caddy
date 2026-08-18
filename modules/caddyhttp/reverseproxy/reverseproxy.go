package reverseproxy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"go.uber.org/zap"
)

func init() {
	caddy.RegisterModule(Handler{})
}

// Handler implements a highly configurable, high-performance
// reverse proxy for HTTP.
type Handler struct {
	TransportRaw json.RawMessage `json:"transport,omitempty" caddy:"namespace=http.reverse_proxy.transport"`
	Backends     UpstreamPool    `json:"backends,omitempty"` 

	// The maximum size of a request body to buffer in memory for retries.
	// If the request body is larger than this, retries will be disabled
	// for that request. Default: 256 KiB.
	MaxRequestBodyBuffer int64 `json:"max_request_body_buffer,omitempty"` 

	logger *zap.Logger
}

// CaddyModule returns the Caddy module information.
func (Handler) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.reverse_proxy",
		New: func() caddy.Module { return new(Handler) },
	}
}

// Provision sets up the handler.
func (h *Handler) Provision(ctx caddy.Context) error {
	h.logger = ctx.Logger()
	if h.MaxRequestBodyBuffer == 0 {
		h.MaxRequestBodyBuffer = 256 * 1024 // 256 KiB
	}
	return nil
}

// UnmarshalCaddyfile sets up the handler from Caddyfile tokens.
func (h *Handler) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	for d.Next() {
		for d.NextBlock(0) {
			switch d.Val() {
			case "max_request_body_buffer":
				if !d.NextArg() {
					return d.ArgErr()
				}
				val, err := caddy.ParseDataSize(d.Val())
				if err != nil {
					return d.Errf("invalid max_request_body_buffer: %v", err)
				}
				h.MaxRequestBodyBuffer = int64(val)
			}
		}
	}
	return nil
}

var bufferPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}
