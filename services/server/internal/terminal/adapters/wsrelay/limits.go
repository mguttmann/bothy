// SPDX-License-Identifier: Apache-2.0

package wsrelay

import (
	"context"
	"net/netip"
	"strconv"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/reqmeta"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// Limiter is the rate limiter of the terminal channels (05 section 3, 02 section 9.1; M2-46
// Design note 10), bound in internal/wire to platform/ratelimit. Every method is safe for
// concurrent use and never waits on the network; a rejection counts in bothy.ratelimit.rejected
// with its class (the limiter does that).
type Limiter interface {
	// AllowTerminal takes one token of rl.terminal of the human principal id (the device stream at
	// identify, after the credential check, before authorize). It returns whether the stream may go
	// on and else the seconds until a token is back.
	AllowTerminal(id ids.ID) (ok bool, retryAfterSeconds int)
	// AllowHost takes one token of rl.host of the host id (control and data connections, after the
	// proof check).
	AllowHost(id ids.ID) (ok bool, retryAfterSeconds int)
	// ReserveCredentials reserves one token of rl.public_auth of the client address before
	// presented credentials are checked (the rule of 02 section 9.1 on the WebSocket channels of the
	// terminal, ruling D-M2-46-11): ok false means the bucket is empty and nothing may be checked;
	// settle(failed) keeps the token for a failed check and gives it back otherwise.
	ReserveCredentials(ip netip.Addr) (ok bool, retryAfterSeconds int, settle func(failed bool))
}

// NoLimiter allows everything; the relay tests of M2-43 and M2-44 run without limits.
type NoLimiter struct{}

// AllowTerminal implements Limiter.
func (NoLimiter) AllowTerminal(ids.ID) (bool, int) { return true, 0 }

// AllowHost implements Limiter.
func (NoLimiter) AllowHost(ids.ID) (bool, int) { return true, 0 }

// ReserveCredentials implements Limiter.
func (NoLimiter) ReserveCredentials(netip.Addr) (bool, int, func(bool)) { return true, 0, func(bool) {} }

// retryText is the close text of 4008: the wait time as the gateway writes it (retry_after=<s>).
func retryText(seconds int) string { return "retry_after=" + strconv.Itoa(seconds) }

// clientIP returns the client address of the request of ctx (kernel/reqmeta); invalid without one,
// which the limiter counts in one shared bucket.
func clientIP(ctx context.Context) netip.Addr {
	ip, _ := reqmeta.ClientIP(ctx)
	return ip
}

// failedHostProof reports the close codes of a failed check of a host proof (4110 unknown key,
// 4113 invalid proof), which consume rl.public_auth.
func failedHostProof(code domain.CloseCode) bool {
	return code == domain.CodeHostUnpaired || code == domain.CodeHostAuthFailed
}
