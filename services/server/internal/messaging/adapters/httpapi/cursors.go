// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"github.com/OWNER/bothy/services/server/internal/messaging/app"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
)

// Cursors is the codec of the list cursors (platform/httpserver), the outgoing port app.Cursors
// of listMyMentions: decoding is an adapter concern.
type Cursors struct{}

// Compile-time check: the codec is the port of the use cases.
var _ app.Cursors = Cursors{}

// Encode implements app.Cursors with httpserver.EncodeCursor.
func (Cursors) Encode(list string, binding []byte, key any) (string, error) {
	return httpserver.EncodeCursor(list, binding, key)
}

// Decode implements app.Cursors with httpserver.DecodeCursor (400 cursor_invalid on a mismatch).
func (Cursors) Decode(list string, binding []byte, s string, key any) error {
	return httpserver.DecodeCursor(list, binding, s, key)
}
