// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"sync/atomic"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// LazyAttachments is the port files.Attachments for messaging and runs (M3-07), bound to the
// files module once its start step has run: the chat step builds messaging and runs before the
// files step, which needs chat (start order of serve.go). Until Bind every method fails with an
// internal error; a message without attachments never calls it, and the server accepts requests
// only after every step ran. It is safe for concurrent use.
type LazyAttachments struct {
	p atomic.Pointer[files.Attachments]
}

// Compile-time check: LazyAttachments is the port.
var _ files.Attachments = (*LazyAttachments)(nil)

// Bind sets the attachments of the files module; a second call replaces the first.
func (l *LazyAttachments) Bind(a files.Attachments) { l.p.Store(&a) }

// get returns the bound port or the internal error of an unbound one.
func (l *LazyAttachments) get() (files.Attachments, error) {
	if p := l.p.Load(); p != nil && *p != nil {
		return *p, nil
	}
	return nil, errs.New(errs.CodeInternalError, "wire: the attachments of the files module are not bound yet")
}

// Attach implements files.Attachments.
func (l *LazyAttachments) Attach(ctx context.Context, cmd files.AttachCmd) ([]files.Attachment, error) {
	a, err := l.get()
	if err != nil {
		return nil, err
	}
	return a.Attach(ctx, cmd)
}

// ForMessages implements files.Attachments.
func (l *LazyAttachments) ForMessages(ctx context.Context, messageIDs []ids.ID) (map[ids.ID][]files.Attachment, error) {
	a, err := l.get()
	if err != nil {
		return nil, err
	}
	return a.ForMessages(ctx, messageIDs)
}

// DeleteForMessage implements files.Attachments.
func (l *LazyAttachments) DeleteForMessage(ctx context.Context, messageID ids.ID) error {
	a, err := l.get()
	if err != nil {
		return err
	}
	return a.DeleteForMessage(ctx, messageID)
}

// MessageOf implements files.Attachments.
func (l *LazyAttachments) MessageOf(ctx context.Context, fileID ids.ID) (files.AttachmentRef, bool, error) {
	a, err := l.get()
	if err != nil {
		return files.AttachmentRef{}, false, err
	}
	return a.MessageOf(ctx, fileID)
}

// Pending implements files.Attachments.
func (l *LazyAttachments) Pending(ctx context.Context, messageID ids.ID) (bool, error) {
	a, err := l.get()
	if err != nil {
		return false, err
	}
	return a.Pending(ctx, messageID)
}
