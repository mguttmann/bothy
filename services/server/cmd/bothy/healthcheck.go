// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/egress"
)

// healthcheckUsage is the help of "bothy healthcheck" (docs/spec/02-architektur.md "Befehle des
// Binarys", 16-betrieb.md "Zwei Listener", 18-release.md "Server-Images").
const healthcheckUsage = `Usage: bothy healthcheck --url URL [--timeout DURATION]

Probes a health endpoint of the internal listener for a container without shell or curl, for
example http://127.0.0.1:8081/readyz. It needs no configuration file, no secrets and no
database, and follows no redirect.

--url URL           http:// URL whose host is a loopback address (127.0.0.0/8, ::1) or
                    localhost; anything else is refused
--timeout DURATION  limit of the whole request (default 5s, the timeout of the Compose
                    healthcheck)

Exit codes: 0 the endpoint answered 2xx, 1 any other answer or no answer (one line on stderr
names the status code or the error class), 2 usage error.
`

// defaultHealthcheckTimeout matches the timeout of x-server-healthcheck in
// infra/compose/compose.yaml; a test holds both.
const defaultHealthcheckTimeout = 5 * time.Second

// healthcheckCommand runs "bothy healthcheck" with the arguments after the command name.
func healthcheckCommand(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("bothy healthcheck", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	rawURL := fs.String("url", "", "loopback URL")
	timeout := fs.Duration("timeout", defaultHealthcheckTimeout, "request limit")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, _ = io.WriteString(stdout, healthcheckUsage)
			return exitOK
		}
		return healthcheckUsageError(stderr, printable(err.Error()))
	}
	if fs.NArg() > 0 {
		return healthcheckUsageError(stderr, fmt.Sprintf("unexpected argument %q", printable(fs.Arg(0))))
	}
	if *timeout <= 0 {
		return healthcheckUsageError(stderr, "--timeout must be positive")
	}
	target, err := loopbackURL(*rawURL)
	if err != nil {
		return healthcheckUsageError(stderr, err.Error())
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return healthcheckUsageError(stderr, "--url is not a valid URL")
	}
	resp, err := egress.NewLoopback(*timeout).Do(req)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "bothy healthcheck: %s\n", errorClass(err))
		return exitFailure
	}
	defer func() { _ = resp.Body.Close() }()
	// Read a little of the body so the server sees a complete exchange; the content is unused.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		_, _ = fmt.Fprintf(stderr, "bothy healthcheck: status %d\n", resp.StatusCode)
		return exitFailure
	}
	return exitOK
}

// loopbackURL checks that raw is an http URL of a loopback host without user info (D5 of the
// brief: the command must not become a general HTTP client). The error is the usage line; it
// quotes nothing of raw.
func loopbackURL(raw string) (string, error) {
	if raw == "" {
		return "", errors.New("--url is required")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", errors.New("--url is not a valid URL")
	}
	if u.Scheme != "http" {
		return "", errors.New("--url must start with http://")
	}
	if u.User != nil {
		return "", errors.New("--url must not carry user info")
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host == "localhost" {
		return u.String(), nil
	}
	addr, err := netip.ParseAddr(host)
	if err != nil || !addr.Unmap().IsLoopback() || addr.Zone() != "" {
		return "", errors.New("--url must name a loopback address (127.0.0.0/8, ::1) or localhost")
	}
	return u.String(), nil
}

// errorClass names the class of a failed request without its text, which may carry addresses.
func errorClass(err error) string {
	var blocked *egress.BlockedError
	var netErr net.Error
	switch {
	case errors.As(err, &blocked):
		return "refused: " + string(blocked.Reason)
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &netErr) && netErr.Timeout():
		return "timeout"
	}
	return "connection failed"
}

// healthcheckUsageError prints msg and the usage on stderr and returns the usage exit code.
func healthcheckUsageError(stderr io.Writer, msg string) int {
	_, _ = fmt.Fprintf(stderr, "bothy healthcheck: %s\n", msg)
	_, _ = io.WriteString(stderr, healthcheckUsage)
	return exitUsage
}
