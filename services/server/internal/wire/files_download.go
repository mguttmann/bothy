// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"encoding/hex"
	"errors"
	"strings"

	filesapp "github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
)

// ticketKeySize is the length of file_ticket_key in docs/operations/configuration.md "5. Secrets"
// (generated, 32 bytes as hex): the start check refuses a shorter key (D-M3-05-15).
const ticketKeySize = 32

// errTicketKey reports a ticket key that is not at least 32 bytes as hex; it never names the value.
var errTicketKey = errors.New("wire: file.ticket_key_ref does not hold at least 32 bytes as hex")

// DownloadConfig binds the keys of downloads, tickets and transcripts (file.ticket_ttl,
// file.not_ready_retry_after, client.cache_max_age). The error names the key whose binding failed,
// never a value.
func DownloadConfig(v *config.Values) (filesapp.DownloadConfig, error) {
	var c filesapp.DownloadConfig
	if err := config.Bind(v, &c); err != nil {
		return filesapp.DownloadConfig{}, err
	}
	return c, nil
}

// TicketKeys is the source of the key of the download tickets (filesapp.TicketKeySource): the
// secret file.ticket_key_ref through the provider of the process, which caches it for
// secrets.cache_ttl, so a replaced key file reaches the process after at most that time. The
// version is the provider's, never derived from the value in a way that reveals it.
type TicketKeys struct {
	sp  *secrets.Provider
	ref string
}

// NewTicketKeys returns the key source over sp and the reference raw (file.ticket_key_ref) and
// reads the key once: a missing, unreadable or short key is an error, so the roles api and all do
// not start without it (D-M3-05-15).
func NewTicketKeys(ctx context.Context, sp *secrets.Provider, raw string) (*TicketKeys, error) {
	if sp == nil {
		return nil, errors.New("wire: file.ticket_key_ref: no secret provider to resolve it")
	}
	k := &TicketKeys{sp: sp, ref: raw}
	if _, _, err := k.TicketKey(ctx); err != nil {
		return nil, err
	}
	return k, nil
}

// TicketKey implements filesapp.TicketKeySource: the hex-decoded key and its version. Errors: the
// provider cannot read the secret, or errTicketKey; no error contains the secret.
func (k *TicketKeys) TicketKey(ctx context.Context) ([]byte, string, error) {
	v, version, err := k.sp.Resolve(ctx, k.ref)
	if err != nil {
		return nil, "", err
	}
	raw, err := hex.DecodeString(strings.TrimSpace(string(v.Bytes())))
	if err != nil || len(raw) < ticketKeySize {
		return nil, "", errTicketKey
	}
	return raw, string(version), nil
}

// sessionActors gives the files module the actor of a ticket through identity.Auth.SessionActor
// (M3-05). The session use cases exist only after the start step sessions, which runs after files,
// so the port is looked up on every call.
type sessionActors struct{ id *Identity }

// errNoSessions reports a ticket fetch before the start step sessions bound the session use cases.
var errNoSessions = errors.New("wire: the session use cases are not bound yet")

// SessionActor implements filesapp.SessionActors.
func (s sessionActors) SessionActor(ctx context.Context, userID, sessionID ids.ID) (actor.Actor, error) {
	if s.id == nil || s.id.Sessions == nil {
		return actor.Actor{}, errNoSessions
	}
	return s.id.Sessions.SessionActor(ctx, userID, sessionID)
}

// EnableDownloads adds downloads, tickets and transcripts to the files module (roles api and all,
// M3-05): the read path over its repository and object store, the actor of a ticket from id, and
// the key source keys. Errors: a missing collaborator or a duration the use cases refuse.
func (f *Files) EnableDownloads(cfg filesapp.DownloadConfig, id *Identity, keys filesapp.TicketKeySource) error {
	return f.Service.EnableDownloads(cfg, filesapp.DownloadDeps{Reader: f.repo, Store: f.store, Sessions: sessionActors{id: id}, Keys: keys})
}
