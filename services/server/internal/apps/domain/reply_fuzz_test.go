// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"testing"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/apps/domain"
)

// The member names of MessageCreate and ComponentCreate in docs/api/openapi.yaml, listed here a
// second time so the fuzz check does not trust the decoder's own lists.
var (
	fuzzMessageMembers = map[string]bool{
		"client_msg_id": true, "kind": true, "content_md": true, "reply_to_message_id": true, "attachment_file_ids": true,
		"mention_ids": true, "silent": true, "clear_draft": true, "run_id": true, "components": true,
	}
	fuzzComponentMembers = map[string]bool{
		"custom_id": true, "type": true, "definition": true, "single_use": true, "required_capability": true,
		"restricted_to": true, "expires_at": true,
	}
)

// FuzzDecodeAppReply: DecodeAppReply never panics, and a body it accepts is valid UTF-8, one JSON
// value without null and without a repeated member name at any depth, and names only members of
// MessageCreate and ComponentCreate (M2-25 design note 9 b 5). The rejected bodies of
// TestDecodeAppReply are seeds and stay rejected.
func FuzzDecodeAppReply(f *testing.F) {
	f.Add([]byte(fullReply))
	f.Add([]byte(`{"client_msg_id":"` + replyMsgID + `","content_md":"ok"}`))
	f.Add([]byte(`{"client_msg_id":"` + replyMsgID + `","content_md":"a","content_md":"b"}`))
	f.Add([]byte(`{"client_msg_id":"` + replyMsgID + `","content_md":null}`))
	f.Add([]byte(`{"client_msg_id":"` + replyMsgID + `","origin":"system"}`))
	f.Add([]byte(`{"Client_Msg_ID":"` + replyMsgID + `"}`))
	f.Add([]byte(`{"client_msg_id":"` + replyMsgID + `","kind":"card","components":[{"custom_id":"a","type":"button","definition":{"x":{"y":[1,{"z":2,"z":3}]}}}]}`))
	f.Add([]byte(`{"choices":[{"name":"a","value":"a"}]}`))
	f.Fuzz(func(t *testing.T, body []byte) {
		r, err := domain.DecodeAppReply(body)
		if err != nil {
			if !errors.Is(err, domain.ErrInvalidReply) {
				t.Fatalf("error %v does not wrap ErrInvalidReply", err)
			}
			return
		}
		if !utf8.Valid(body) {
			t.Fatal("accepted invalid UTF-8")
		}
		if r.ClientMsgID.IsZero() {
			t.Fatal("accepted a body without client_msg_id")
		}
		if err := independentCheck(body); err != nil {
			t.Fatalf("accepted a body that the independent check refuses: %v", err)
		}
	})
}

// independentCheck parses body recursively and refuses a null, a repeated member name, a member
// outside MessageCreate at the top, a member outside ComponentCreate in an element of
// components, and trailing data.
func independentCheck(body []byte) error {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := checkValue(dec, levelMessage); err != nil {
		return err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return errors.New("trailing data")
	}
	return nil
}

// level says which member list an object at this place must keep to.
type level int

const (
	levelFree level = iota
	levelMessage
	levelComponents
	levelComponent
)

// checkValue reads one value at the place lvl.
func checkValue(dec *json.Decoder, lvl level) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	if tok == nil {
		return errors.New("null")
	}
	d, ok := tok.(json.Delim)
	if !ok {
		return nil
	}
	if d == '[' {
		inner := levelFree
		if lvl == levelComponents {
			inner = levelComponent
		}
		for dec.More() {
			if err := checkValue(dec, inner); err != nil {
				return err
			}
		}
		_, err := dec.Token()
		return err
	}
	seen := map[string]bool{}
	for dec.More() {
		k, err := dec.Token()
		if err != nil {
			return err
		}
		name, _ := k.(string)
		if seen[name] {
			return errors.New("repeated member")
		}
		seen[name] = true
		next := levelFree
		switch lvl {
		case levelMessage:
			if !fuzzMessageMembers[name] {
				return errors.New("member outside MessageCreate")
			}
			if name == "components" {
				next = levelComponents
			}
		case levelComponent:
			if !fuzzComponentMembers[name] {
				return errors.New("member outside ComponentCreate")
			}
		case levelFree, levelComponents:
			// Any member name; levelComponents is an array, so no object stands at it.
		}
		if err := checkValue(dec, next); err != nil {
			return err
		}
	}
	_, err = dec.Token()
	return err
}
