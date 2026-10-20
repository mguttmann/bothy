// SPDX-License-Identifier: Apache-2.0

package incoming

import (
	"strings"
	"unicode"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Native is a native body as received; a nil pointer is an absent member.
type Native struct {
	// ContentMD is the Markdown content.
	ContentMD *string
	// Title asks for a new thread with this title (11 section 8.3 rule 3).
	Title *string
	// ThreadID names an existing thread (11 section 8.3 rule 2).
	ThreadID *ids.ID
	// ClientMsgID makes the post idempotent (04 section 5.1). It holds the 16 bytes as sent, so
	// CheckNative can refuse other UUID versions.
	ClientMsgID *ids.ID
}

// NativeRequest is a checked native body.
type NativeRequest struct {
	// ContentMD is the content, byte for byte as received.
	ContentMD string
	// Title is nil when absent; chat checks its rules with pointer /title.
	Title *string
	// ThreadID is nil when absent.
	ThreadID *ids.ID
	// ClientMsgID is the client's UUIDv7 or a new server UUIDv7.
	ClientMsgID ids.ID
	// ClientGiven is true when the body carried client_msg_id.
	ClientGiven bool
}

// CheckNative applies the body rules of a native post (M2-27 design note 4c, M1-56 D2) with
// maxContentBytes from message.max_content_bytes. Errors: content_md missing gives
// validation_failed at /content_md with required, content without a non-white-space character the
// same with empty; more UTF-8 bytes than maxContentBytes give payload_too_large naming
// message.max_content_bytes; a client_msg_id that is not a UUIDv7 with the RFC variant gives
// validation_failed at /client_msg_id with not_uuid_v7. When client_msg_id is absent newID
// supplies a server UUIDv7 (03 messages.client_msg_id). Title rules are not checked here.
func CheckNative(n Native, maxContentBytes int, newID func() ids.ID) (NativeRequest, error) {
	if n.ContentMD == nil {
		return NativeRequest{}, fieldError("/content_md", CodeRequired, "content_md is required.")
	}
	content := *n.ContentMD
	if len(content) > maxContentBytes {
		return NativeRequest{}, errs.New(errs.CodePayloadTooLarge, "content_md is larger than message.max_content_bytes.")
	}
	if strings.IndexFunc(content, func(r rune) bool { return !unicode.IsSpace(r) }) < 0 {
		return NativeRequest{}, fieldError("/content_md", CodeEmpty, "content_md has no visible character.")
	}
	req := NativeRequest{ContentMD: content, Title: n.Title, ThreadID: n.ThreadID}
	if n.ClientMsgID != nil {
		id := *n.ClientMsgID
		if id[6]>>4 != 7 || id[8]>>6 != 0b10 {
			return NativeRequest{}, fieldError("/client_msg_id", CodeNotUUIDv7, "client_msg_id is not a UUIDv7.")
		}
		req.ClientMsgID, req.ClientGiven = id, true
		return req, nil
	}
	req.ClientMsgID = newID()
	return req, nil
}
