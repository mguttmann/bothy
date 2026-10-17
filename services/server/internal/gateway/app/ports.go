// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"

	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Conn is the outgoing port of one WebSocket connection, implemented by adapters/ws: the
// gateway hands it the frames of this package and the close. Both are safe for concurrent use
// and never block on the network.
type Conn interface {
	// Send queues the frame f (Hello, Ready, Resumed, HeartbeatAck, ReauthRequired,
	// ResyncRequired or ErrorFrame) for the writer. Errors: the connection is closing, or the
	// outbound queue overflowed, in which case the adapter has closed it with 4014.
	Send(f any) error
	// Close closes the connection with code and reason (machine data only, at most 123 bytes).
	// Calls after the first have no effect.
	Close(code domain.CloseCode, reason string)
	// Offer queues the encoded entry e of the hub (dispatch, cursor, catchup.complete) when the
	// outbound queue has room and reports false otherwise; it never blocks and never closes.
	Offer(e domain.Entry) bool
	// WaitSpace blocks until the outbound queue has room, the connection closes (an error) or
	// ctx ends (its error). Only the catch-up runner of the connection waits (04 section 7.2).
	WaitSpace(ctx context.Context) error
	// Purge drops the queued dispatch entries of the removed targets t (04 section 8.3 item 2).
	Purge(t domain.Targets)
}

// FrameDecoder is the outgoing port of the frame codec (adapters/ws): it checks the body d of a
// client op against the schema of the op in the contract and decodes it into the frame type of
// this package (Identify, Resume, Heartbeat, Reauth). Errors: d does not match the schema
// (unknown members included); the dispatcher closes with 4002 then.
type FrameDecoder interface {
	Decode(op string, d json.RawMessage) (any, error)
}

// NonceSource supplies the current server DPoP nonce (identity, M1-33) for hello and
// reauth_required; the current and the previous nonce are accepted (08 "DPoP").
type NonceSource interface {
	CurrentNonce() string
}

// ConnectLimiter is the rate-limit class rl.gateway_connect (platform/ratelimit, M1-19), keyed by
// the verified device of a person or the API token of an agent or app. AllowConnect takes one
// token; when the bucket is empty it reports false and the seconds until one is there. Errors:
// an actor with neither device nor token.
type ConnectLimiter interface {
	AllowConnect(a actor.Actor) (allowed bool, retryAfterSeconds int, err error)
}

// EpochSource reads the current sync epoch of the instance for hello (instance.sync_epoch).
type EpochSource interface {
	SyncEpoch(ctx context.Context) (ids.ID, error)
}

// ProofAlarm counts an op whose handler returned without an authorize check of its declared
// action: bothy.authz.errors with the reason authz_not_checked, the counter of the HTTP proof
// check (02 "Nachweis der Prüfung").
type ProofAlarm interface {
	NotChecked(ctx context.Context, op string, action authorize.Action)
}
