// SPDX-License-Identifier: Apache-2.0

// Command fakepush is the fake push provider of the mobile device E2E test (brief M5-11): the
// Compose service fakepush of infra/compose/test/compose.e2e-mobile.yaml, built as the target
// fakepush of services/server/Dockerfile. See package fakepush for what it serves.
//
// Usage: fakepush serve [flags] | fakepush health [--addr ADDR]
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/OWNER/bothy/services/server/test/fakepush"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	code := fakepush.Run(ctx, os.Args[1:], os.Stderr)
	stop()
	os.Exit(code)
}
