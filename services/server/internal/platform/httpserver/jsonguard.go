// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// Details of malformed_request; they name the rule, never the submitted content.
const (
	detailInvalidUTF8   = "Body is not valid UTF-8."
	detailDuplicate     = "Body has a duplicate member name."
	detailTooDeep       = "Body nests deeper than " + keyMaxJSONDepth + "."
	detailNotJSON       = "Body is not one valid JSON value."
	detailEmptyRequired = "Body is required."
)

// guardFrame is one open object or array while the guard walks the tokens.
type guardFrame struct {
	object    bool
	expectKey bool
	keys      map[string]struct{}
}

// CheckJSON is the JSON guard of SEC-IN-2: body must be valid UTF-8, exactly one JSON value
// without a duplicate member name at any level, and nest no deeper than maxDepth (an object or
// array is one level). Violations are 400 malformed_request with a detail naming the rule. It
// uses encoding/json's tokenizer, so the guard and the decoder of the handler agree on the syntax
// (the same reading for client_payload_sha256 and processing).
func CheckJSON(body []byte, maxDepth int) error {
	if !utf8.Valid(body) {
		return errs.New(errs.CodeMalformedRequest, detailInvalidUTF8)
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var stack []*guardFrame
	values := 0
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) && len(stack) == 0 && values == 1 {
			return nil
		}
		if err != nil {
			return errs.New(errs.CodeMalformedRequest, detailNotJSON)
		}
		if len(stack) == 0 {
			if values == 1 {
				return errs.New(errs.CodeMalformedRequest, detailNotJSON)
			}
			values++
		}
		if e := guardToken(tok, &stack, maxDepth); e != nil {
			return e
		}
	}
}

// guardToken applies one token to the stack of open containers.
func guardToken(tok json.Token, stack *[]*guardFrame, maxDepth int) error {
	d, isDelim := tok.(json.Delim)
	if isDelim && (d == '}' || d == ']') {
		*stack = (*stack)[:len(*stack)-1]
		return nil
	}
	if n := len(*stack); n > 0 && (*stack)[n-1].object {
		top := (*stack)[n-1]
		if top.expectKey {
			return top.key(tok)
		}
		top.expectKey = true // the value of the member; the next token is a key again
	}
	if !isDelim {
		return nil
	}
	if len(*stack) >= maxDepth {
		return errs.New(errs.CodeMalformedRequest, detailTooDeep)
	}
	f := &guardFrame{object: d == '{', expectKey: d == '{'}
	if f.object {
		f.keys = map[string]struct{}{}
	}
	*stack = append(*stack, f)
	return nil
}

// key records the member name tok of the object f; a repeated name is malformed_request.
func (f *guardFrame) key(tok json.Token) error {
	name, _ := tok.(string)
	if _, dup := f.keys[name]; dup {
		return errs.New(errs.CodeMalformedRequest, detailDuplicate)
	}
	f.keys[name] = struct{}{}
	f.expectKey = false
	return nil
}
