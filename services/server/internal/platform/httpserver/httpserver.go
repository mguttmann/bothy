// SPDX-License-Identifier: Apache-2.0

// Package httpserver opens the listeners of the server and builds its http.Server values
// (docs/spec/02-architecture.md "Request path", docs/spec/16-operations.md "Two listeners"). This
// is the listener part of M1-11; the middleware chain, the problem format and the per-route
// deadlines come with M1-18, the plaintext peer check in Listen with M1-21.
//
// Example:
//
//	ln, err := httpserver.Listen(ctx, "127.0.0.1:8081")
//	srv := httpserver.NewServer(cfg, httpserver.Internal(reg), log)
//	go srv.Serve(ln)
package httpserver

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/health"
	"github.com/OWNER/bothy/services/server/internal/platform/otel"
)

// Config holds the server limits of docs/spec/02-architecture.md "Configuration values of this
// chapter"; internal/wire binds it with config.Bind.
type Config struct {
	// ReadHeaderTimeout is http.read_header_timeout: the time to read the request headers.
	ReadHeaderTimeout time.Duration `config:"http.read_header_timeout"`
	// IdleTimeout is http.idle_timeout: how long a keep-alive connection may stay idle.
	IdleTimeout time.Duration `config:"http.idle_timeout"`
	// MaxHeaderBytes is http.max_header_bytes: the largest request header block.
	MaxHeaderBytes int `config:"http.max_header_bytes"`
}

// Listen opens a TCP listener on addr ("host:port"; port 0 picks a free port). It is the one
// place where the server opens listeners, so M1-21 adds the plaintext peer check here. The error
// is the one of net.ListenConfig.Listen and names the address.
func Listen(ctx context.Context, addr string) (net.Listener, error) {
	var lc net.ListenConfig
	return lc.Listen(ctx, "tcp", addr)
}

// NewServer returns the http.Server for h with the limits of cfg. ErrorLog is
// otel.ServerErrorLog(log), so TLS, accept and panic lines of net/http reach the logs as the
// constant "http server error" without the client address (19-security.md SEC-LOG-5). There
// is no ReadTimeout or WriteTimeout: per-route deadlines come with M1-18, and a server-wide
// write deadline would cut long downloads. log nil means slog.Default().
func NewServer(cfg Config, h http.Handler, log *slog.Logger) *http.Server {
	if log == nil {
		log = slog.Default()
	}
	return &http.Server{
		Handler:           h,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		MaxHeaderBytes:    cfg.MaxHeaderBytes,
		ErrorLog:          otel.ServerErrorLog(log),
	}
}

// Internal returns the mux of the internal listener with health.PathLiveness and
// health.PathReadiness of reg. Later tickets may add stack-internal webhook routes (16 "Two
// listeners", Alertmanager) before the listener starts; every other path answers 404.
func Internal(reg *health.Registry) *http.ServeMux {
	mux := http.NewServeMux()
	h := reg.Handler()
	// nosemgrep: bothy-go-route-registration -- health is outside the public API and has no action (R-AZ-22, L2); the route table does not cover the internal listener
	mux.Handle(health.PathLiveness, h)
	// nosemgrep: bothy-go-route-registration -- health is outside the public API and has no action (R-AZ-22, L2); the route table does not cover the internal listener
	mux.Handle(health.PathReadiness, h)
	return mux
}

// Public returns the handler of the public listener: the routes mounted on mux, and 404 with
// an empty body for every request no route matches, so the public listener never reveals the
// internal paths or the default text of net/http. M1-18 replaces the fallback with the problem
// format of its chain.
func Public(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, pattern := mux.Handler(r); pattern == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		mux.ServeHTTP(w, r)
	})
}
