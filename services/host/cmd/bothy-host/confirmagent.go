// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"io"
	"os"

	"github.com/OWNER/bothy/services/host/internal/confirm"
)

// cmdConfirmAgent is "confirm-agent": the confirmation agent of the person at the machine (M2-45
// Design note 15), started in their desktop session by an autostart entry the operator creates
// (16 4.4). It connects to the endpoint of the host service and answers every request with a
// dialog whose default is deny; without a dialog program it denies.
func cmdConfirmAgent(ctx context.Context, args []string, _, stderr io.Writer) int {
	fl := newFlags("confirm-agent", stderr)
	if err := fl.Parse(args); err != nil || fl.NArg() != 0 {
		return exitUsage
	}
	log := sys.log(stderr)
	err := confirm.RunAgent(ctx, confirm.AgentOptions{Endpoint: confirm.Endpoint(sys.goos), Dialog: confirm.NativeDialog(log, os.Environ()), Log: log})
	if err != nil && ctx.Err() == nil {
		_, _ = io.WriteString(stderr, "bothy-host confirm-agent: endpoint unavailable\n")
		return exitFailed
	}
	return exitOK
}
