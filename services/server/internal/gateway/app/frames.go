// SPDX-License-Identifier: Apache-2.0

package app

import (
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// The frames of the gateway as the use cases see them. adapters/ws converts them from and to the
// generated Gateway* types of api/rest (schemas of the same names in docs/api/openapi.yaml); the
// JSON tags let tests build them without the adapter.

// Identify is the body of identify (schema GatewayIdentify). Exactly one of AccessToken (people)
// and APIToken (agents and apps) is set.
type Identify struct {
	ProtocolVersion int         `json:"protocol_version"`
	AccessToken     string      `json:"access_token,omitempty"`
	APIToken        string      `json:"api_token,omitempty"`
	DPoP            string      `json:"dpop,omitempty"`
	DeviceID        ids.ID      `json:"device_id,omitzero"`
	Consumer        *string     `json:"consumer,omitempty"`
	Cursor          *int64      `json:"cursor,omitempty"`
	SyncEpoch       *ids.ID     `json:"sync_epoch,omitempty"`
	Client          *ClientInfo `json:"client,omitempty"`
}

// ClientInfo is the client platform and version of identify (schema GatewayClientInfo).
type ClientInfo struct {
	Platform string `json:"platform"`
	Version  string `json:"version"`
}

// Resume is the body of resume (schema GatewayResume).
type Resume struct {
	SessionID   ids.ID  `json:"gateway_session_id"`
	AccessToken string  `json:"access_token,omitempty"`
	APIToken    string  `json:"api_token,omitempty"`
	DPoP        string  `json:"dpop,omitempty"`
	Cursor      int64   `json:"cursor"`
	SyncEpoch   *ids.ID `json:"sync_epoch,omitempty"`
}

// Heartbeat is the body of heartbeat (schema GatewayHeartbeat). Agent reports that the member
// agent was sent; it is accepted and ignored until M2-20.
type Heartbeat struct {
	LastSeq int64 `json:"last_seq"`
	Agent   bool  `json:"-"`
}

// Reauth is the body of reauth (schema GatewayReauth).
type Reauth struct {
	AccessToken string `json:"access_token"`
	DPoP        string `json:"dpop"`
}

// Ack is the body of ack (schema GatewayAck): the acknowledgement of an agent consumer, valid for
// every number up to Seq (04 section 15.2 item 1).
type Ack struct {
	Seq int64 `json:"seq"`
}

// Focus is the body of focus (schema GatewayFocus): the complete set of the threads on screen,
// which replaces the previous one, and idle.
type Focus struct {
	ThreadIDs []ids.ID `json:"thread_ids"`
	Idle      bool     `json:"idle,omitempty"`
}

// Typing is the body of typing.start and typing.stop (schema GatewayTyping).
type Typing struct {
	ThreadID ids.ID `json:"thread_id"`
}

// Hello is the first frame after the upgrade (schema GatewayHello).
type Hello struct {
	HeartbeatInterval time.Duration
	ResumeWindow      time.Duration
	ProtocolVersions  []int
	DPoPNonce         string
	ServerTime        time.Time
	SyncEpoch         ids.ID
}

// Ready answers a successful identify (schema GatewayReady): the session, the device of a person
// (zero for agents and apps) and the snapshot, whose principal, permission version, head and
// epoch the frame repeats.
type Ready struct {
	SessionID ids.ID
	DeviceID  ids.ID
	View      sync.SnapshotView
}

// Resumed answers a successful resume (schema GatewayResumed).
type Resumed struct {
	SessionID ids.ID
	HeadSeq   int64
	SyncEpoch ids.ID
}

// HeartbeatAck answers a heartbeat (schema GatewayHeartbeatAck).
type HeartbeatAck struct {
	ServerTime time.Time
}

// ReauthRequired announces the expiry of a person's access token (schema GatewayReauthRequired).
type ReauthRequired struct {
	ExpiresAt time.Time
	DPoPNonce string
}

// ResyncRequired tells the client after ready or resumed that catch-up from its cursor is not
// possible; the connection stays live from HeadSeq (schema GatewayResyncRequired, 04 section 7.3).
type ResyncRequired struct {
	Reason    sync.ResyncReason
	HeadSeq   int64
	SyncEpoch ids.ID
}

// ErrorFrame answers an op that failed without ending the connection (schema GatewayError): the
// problem code of 06, an optional detail and the ref of the op.
type ErrorFrame struct {
	Op, Code, Detail, Ref string
}
