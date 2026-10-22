// SPDX-License-Identifier: Apache-2.0

package incoming_test

import (
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/apps/domain/incoming"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

func ptr[T any](v T) *T { return &v }

func TestNativeRules(t *testing.T) {
	const maxBytes = 64
	serverID := ids.New()
	newID := func() ids.ID { return serverID }

	t.Run("content kept byte for byte, absent client_msg_id gives a server UUIDv7", func(t *testing.T) {
		content := "  **Report** for @alex\n"
		got, err := incoming.CheckNative(incoming.Native{ContentMD: ptr(content), Title: ptr("Nightly")}, maxBytes, newID)
		if err != nil {
			t.Fatal(err)
		}
		if got.ContentMD != content || got.ClientMsgID != serverID || got.ClientGiven || got.Title == nil || *got.Title != "Nightly" || got.ThreadID != nil {
			t.Fatalf("got %+v", got)
		}
		if serverID[6]>>4 != 7 {
			t.Fatal("the server id is no UUIDv7")
		}
	})

	t.Run("client_msg_id of the client is kept", func(t *testing.T) {
		client := ids.New()
		thread := ids.New()
		got, err := incoming.CheckNative(incoming.Native{ContentMD: ptr("x"), ClientMsgID: &client, ThreadID: &thread}, maxBytes, newID)
		if err != nil {
			t.Fatal(err)
		}
		if got.ClientMsgID != client || !got.ClientGiven || got.ThreadID == nil || *got.ThreadID != thread {
			t.Fatalf("got %+v", got)
		}
	})

	t.Run("missing content", func(t *testing.T) {
		_, err := incoming.CheckNative(incoming.Native{}, maxBytes, newID)
		wantField(t, err, "/content_md", incoming.CodeRequired)
	})

	for _, content := range []string{"", " ", "\n\t  "} {
		t.Run("empty content", func(t *testing.T) {
			_, err := incoming.CheckNative(incoming.Native{ContentMD: ptr(content)}, maxBytes, newID)
			wantField(t, err, "/content_md", incoming.CodeEmpty)
		})
	}

	t.Run("content size", func(t *testing.T) {
		if _, err := incoming.CheckNative(incoming.Native{ContentMD: ptr(strings.Repeat("a", maxBytes))}, maxBytes, newID); err != nil {
			t.Fatalf("exactly the limit: %v", err)
		}
		_, err := incoming.CheckNative(incoming.Native{ContentMD: ptr(strings.Repeat("a", maxBytes+1))}, maxBytes, newID)
		e, ok := errs.As(err)
		if !ok || e.Code != errs.CodePayloadTooLarge || !strings.Contains(e.Detail, "message.max_content_bytes") {
			t.Fatalf("one byte more: %v", err)
		}
	})

	t.Run("client_msg_id of version 4 rejected", func(t *testing.T) {
		v4 := ids.New()
		v4[6] = v4[6]&0x0f | 0x40
		_, err := incoming.CheckNative(incoming.Native{ContentMD: ptr("x"), ClientMsgID: &v4}, maxBytes, newID)
		wantField(t, err, "/client_msg_id", incoming.CodeNotUUIDv7)
	})

	t.Run("client_msg_id with another variant rejected", func(t *testing.T) {
		v := ids.New()
		v[8] = v[8]&0x3f | 0xc0
		_, err := incoming.CheckNative(incoming.Native{ContentMD: ptr("x"), ClientMsgID: &v}, maxBytes, newID)
		wantField(t, err, "/client_msg_id", incoming.CodeNotUUIDv7)
	})
}
