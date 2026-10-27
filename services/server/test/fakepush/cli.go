// SPDX-License-Identifier: Apache-2.0

package fakepush

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"time"
)

// usage is the help of the command fakepush (cmd/fakepush).
const usage = `Usage: fakepush serve [flags] | fakepush health [--addr ADDR]

serve   generate the run material into --dir, start Fake-APNs, Fake-FCM and the capture API,
        print one ready line, run until SIGTERM or SIGINT
        --dir DIR            directory of the files (default /push)
        --host NAME          host name of the fakes in certificate and URLs (default fakepush)
        --apns-addr ADDR     listen address of Fake-APNs (default :8443)
        --fcm-addr ADDR      listen address of Fake-FCM (default :8444)
        --capture-addr ADDR  listen address of the capture API (default :8090)
health  exit 0 when GET http://ADDR/healthz answers 200, else 1 (the Compose healthcheck)
        --addr ADDR          address of the capture API (default 127.0.0.1:8090)

Exit codes: 0 success, 1 failure, 2 usage error.
`

// Exit codes of Run.
const (
	exitOK      = 0
	exitFailure = 1
	exitUsage   = 2
)

// healthTimeout bounds the health probe; the Compose healthcheck has its own timeout above it.
const healthTimeout = 3 * time.Second

// Run runs the command fakepush with args (without the program name) and returns its exit code:
// "serve" runs until ctx ends and then closes everything it started (0), "health" probes the
// capture API (0 healthy, 1 not). Messages go to stderr; they name addresses and errors, never a
// key, token or payload.
func Run(ctx context.Context, args []string, stderr io.Writer) int {
	if len(args) == 0 {
		return usageError(stderr, "a subcommand is required")
	}
	switch args[0] {
	case "serve":
		return serve(ctx, args[1:], stderr)
	case "health":
		return health(ctx, args[1:], stderr)
	default:
		return usageError(stderr, fmt.Sprintf("unknown subcommand %q", args[0]))
	}
}

// serve parses the flags of serve, starts a Server and waits for the end of ctx.
func serve(ctx context.Context, args []string, stderr io.Writer) int {
	fs := flag.NewFlagSet("fakepush serve", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var opts Options
	fs.StringVar(&opts.Dir, "dir", "/push", "directory of the files")
	fs.StringVar(&opts.Host, "host", "fakepush", "host name of the fakes")
	fs.StringVar(&opts.APNsAddr, "apns-addr", ":8443", "listen address of Fake-APNs")
	fs.StringVar(&opts.FCMAddr, "fcm-addr", ":8444", "listen address of Fake-FCM")
	fs.StringVar(&opts.CaptureAddr, "capture-addr", ":8090", "listen address of the capture API")
	if err := fs.Parse(args); err != nil {
		return usageError(stderr, err.Error())
	}
	if fs.NArg() > 0 {
		return usageError(stderr, fmt.Sprintf("unexpected argument %q", fs.Arg(0)))
	}
	s, err := Start(opts)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "fakepush: %v\n", err)
		return exitFailure
	}
	_, _ = fmt.Fprintf(stderr, "fakepush: ready apns=%s fcm=%s capture=%s\n", s.APNsURL(), s.FCMURL(), s.CaptureURL())
	<-ctx.Done()
	if err := s.Close(); err != nil {
		_, _ = fmt.Fprintf(stderr, "fakepush: stop: %v\n", err)
		return exitFailure
	}
	_, _ = fmt.Fprintln(stderr, "fakepush: stopped")
	return exitOK
}

// health probes GET http://<addr>/healthz once.
func health(ctx context.Context, args []string, stderr io.Writer) int {
	fs := flag.NewFlagSet("fakepush health", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	addr := fs.String("addr", "127.0.0.1:8090", "address of the capture API")
	if err := fs.Parse(args); err != nil {
		return usageError(stderr, err.Error())
	}
	if fs.NArg() > 0 {
		return usageError(stderr, fmt.Sprintf("unexpected argument %q", fs.Arg(0)))
	}
	ctx, cancel := context.WithTimeout(ctx, healthTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+*addr+"/healthz", nil)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "fakepush health: %v\n", err)
		return exitFailure
	}
	// The capture API is plain HTTP on the address of this container; no proxy, no redirect.
	c := &http.Client{Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := c.Do(req)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "fakepush health: %v\n", errors.Unwrap(err))
		return exitFailure
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = fmt.Fprintf(stderr, "fakepush health: status %d\n", resp.StatusCode)
		return exitFailure
	}
	return exitOK
}

// usageError prints msg and the usage and returns the usage exit code.
func usageError(stderr io.Writer, msg string) int {
	_, _ = fmt.Fprintf(stderr, "fakepush: %s\n", msg)
	_, _ = io.WriteString(stderr, usage)
	return exitUsage
}
