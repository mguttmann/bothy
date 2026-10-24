// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"sync"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TicketKeySource returns the current key of the download tickets and its version
// (file.ticket_key_ref through the secret provider of M1-09, implemented in internal/wire). The
// provider caches the value for secrets.cache_ttl, so a key change reaches a process after at
// most that time. Errors: the provider cannot read the secret, or the value is not a key.
type TicketKeySource interface {
	TicketKey(ctx context.Context) (key []byte, version string, err error)
}

// keyRing keeps the current ticket key and the one before it (12 9.2 point 2: two keys are valid
// at the same time). It mints with the current key and verifies with both; on a version change the
// former current key becomes the previous one, so tickets minted before keep working until they
// expire (D-M3-05-1). It is safe for concurrent use.
type keyRing struct {
	src TicketKeySource

	mu      sync.Mutex
	version string
	current []byte
	prev    []byte
}

// newKeyRing returns a ring over src; the first call reads the key.
func newKeyRing(src TicketKeySource) *keyRing { return &keyRing{src: src} }

// keys reads the current key from the source and returns the current and the previous key.
func (r *keyRing) keys(ctx context.Context) (current, previous []byte, err error) {
	key, version, err := r.src.TicketKey(ctx)
	if err != nil {
		return nil, nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if version != r.version || r.current == nil {
		if r.current != nil {
			r.prev = r.current
		}
		r.current, r.version = key, version
	}
	return r.current, r.prev, nil
}

// verify decodes a ticket under the current and the previous key at now.
func (r *keyRing) verify(ctx context.Context, ticket string, now time.Time) (domain.Ticket, error) {
	cur, prev, err := r.keys(ctx)
	if err != nil {
		return domain.Ticket{}, err
	}
	return domain.DecodeTicket([][]byte{cur, prev}, ticket, now)
}

// mint encodes t with the current key.
func (r *keyRing) mint(ctx context.Context, t domain.Ticket) (string, error) {
	cur, _, err := r.keys(ctx)
	if err != nil {
		return "", err
	}
	return domain.EncodeTicket(cur, t), nil
}

// The relative references of a ticket (openapi FileTicket.url).
const (
	ticketPathPrefix   = "/api/v1/files/"
	ticketPathContent  = "/content?ticket="
	ticketPathThumbs   = "/thumbnails/"
	ticketQueryOfThumb = "?ticket="
)

// IssueTicket implements files.Files (createFileTicket; brief M3-05 design note 3): the file (404
// for unknown or deleted), file.read with the descriptor fact IssueTicket (a principal without a
// login session gets 403 step_up_not_possible), the readiness (no ticket for a file that cannot
// be delivered, D-M3-05-7) and the variant (404 when the file has none); then a ticket with the
// current key, valid for file.ticket_ttl in whole seconds. A repeated call mints a new ticket.
func (s *Service) IssueTicket(ctx context.Context, fileID ids.ID, variant string) (files.IssuedTicket, error) {
	if s.dl == nil {
		return files.IssuedTicket{}, errDownloadsOff
	}
	a := caller(ctx)
	if a.IsAnonymous() {
		return files.IssuedTicket{}, s.anonymous(ctx, a, authorize.ActionFileRead, fileID)
	}
	v, ok := domain.ParseVariant(variant)
	if !ok {
		return files.IssuedTicket{}, fieldError("/variant", codeInvalid, "unknown variant")
	}
	f, err := s.readable(ctx, a, fileID, true, false)
	if err != nil {
		return files.IssuedTicket{}, err
	}
	if v.IsThumbnail() {
		if _, found, err := s.dl.d.Reader.Thumbnail(ctx, f.ID, string(v)); err != nil || !found {
			if err != nil {
				return files.IssuedTicket{}, err
			}
			return files.IssuedTicket{}, notFound()
		}
	}
	if a.SessionID.IsZero() {
		// authorize refuses a principal without a session (step_up_not_possible); this guard only
		// keeps a ticket from ever binding to no session.
		return files.IssuedTicket{}, errs.New(errs.CodeInternalError, "files: a ticket without a session was allowed")
	}
	exp := s.d.Clock.Now().Add(s.dl.cfg.TicketTTL).UTC().Truncate(time.Second)
	t, err := s.dl.ring.mint(ctx, domain.Ticket{FileID: f.ID, Variant: v, PrincipalID: a.ID, SessionID: a.SessionID, ExpiresAt: exp})
	if err != nil {
		return files.IssuedTicket{}, err
	}
	url := ticketPathPrefix + f.ID.String() + ticketPathContent + t
	if v.IsThumbnail() {
		url = ticketPathPrefix + f.ID.String() + ticketPathThumbs + string(v) + ticketQueryOfThumb + t
	}
	return files.IssuedTicket{URL: url, ExpiresAt: exp}, nil
}
