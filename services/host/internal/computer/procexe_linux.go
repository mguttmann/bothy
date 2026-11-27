// SPDX-License-Identifier: Apache-2.0

package computer

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/OWNER/bothy/services/host/internal/confirm"
)

// errNoSignature is the signature check of a platform without a helper.
var errNoSignature = errors.New("computer: no signature check on this platform")

// DefaultBind binds nothing on Linux: the peer is the uid that SO_PEERCRED took at connect time,
// and the path and signature parts of the peer rule do not apply (docs/spec/27-computer-use.md
// 9.4).
func DefaultBind(net.Conn, confirm.Peer, time.Time) (Process, error) { return nil, nil } //nolint:nilnil // no binding is the Linux rule

// DefaultVerifySignature refuses: Linux has no signed helper.
func DefaultVerifySignature(context.Context, string, string) error { return errNoSignature }
