// SPDX-License-Identifier: Apache-2.0

package domain_test

// An RFC 6902 applier for the tests, written from RFC 6902 sections 4.1 (add), 4.2 (remove) and
// 4.3 (replace) with RFC 6901 pointers. It shares no code with the package under test on
// purpose: the property tests use it to prove that the normalized operations of Apply turn the
// document at the base version into the document at the new version, and a shared pointer
// parser would hide a bug in both.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// rfcPointer splits a JSON Pointer (RFC 6901 section 3 and 4) into reference tokens.
func rfcPointer(p string) ([]string, error) {
	if p == "" {
		return nil, nil
	}
	if p[0] != '/' {
		return nil, errors.New("pointer does not start with /")
	}
	parts := strings.Split(p[1:], "/")
	for i, part := range parts {
		var b strings.Builder
		for j := 0; j < len(part); j++ {
			if part[j] != '~' {
				b.WriteByte(part[j])
				continue
			}
			if j+1 >= len(part) {
				return nil, errors.New("dangling ~")
			}
			switch part[j+1] {
			case '0':
				b.WriteByte('~')
			case '1':
				b.WriteByte('/')
			default:
				return nil, errors.New("bad escape")
			}
			j++
		}
		parts[i] = b.String()
	}
	return parts, nil
}

// rfcDecode decodes a JSON value with float64 numbers.
func rfcDecode(raw []byte) (any, error) {
	var v any
	err := json.Unmarshal(raw, &v)
	return v, err
}

// rfcApply applies ops to doc in order and returns the new document. A failing operation fails
// the whole patch (RFC 6902 section 5).
func rfcApply(doc any, ops []domain.NormOp) (any, error) {
	for i, op := range ops {
		toks, err := rfcPointer(op.Path)
		if err != nil {
			return nil, fmt.Errorf("op %d: %w", i, err)
		}
		var val any
		if op.Op != domain.OpRemove {
			if val, err = rfcDecode(op.Value); err != nil {
				return nil, fmt.Errorf("op %d: value: %w", i, err)
			}
		}
		if len(toks) == 0 {
			// The whole document: add and replace set it, remove is not used by the server.
			if op.Op == domain.OpRemove {
				return nil, fmt.Errorf("op %d: remove of the root", i)
			}
			doc = val
			continue
		}
		parent, err := rfcWalk(doc, toks[:len(toks)-1])
		if err != nil {
			return nil, fmt.Errorf("op %d: %w", i, err)
		}
		last := toks[len(toks)-1]
		switch p := parent.(type) {
		case map[string]any:
			_, exists := p[last]
			switch op.Op {
			case domain.OpAdd: // 4.1: adds or replaces the member.
				p[last] = val
			case domain.OpRemove: // 4.2: the target must exist.
				if !exists {
					return nil, fmt.Errorf("op %d: remove of missing %s", i, op.Path)
				}
				delete(p, last)
			case domain.OpReplace: // 4.3: the target must exist.
				if !exists {
					return nil, fmt.Errorf("op %d: replace of missing %s", i, op.Path)
				}
				p[last] = val
			default:
				return nil, fmt.Errorf("op %d: unknown op %q", i, op.Op)
			}
		case []any:
			// Arrays only appear inside values the server sends whole; it never addresses an
			// element, so an array parent is a failure of the normalized ops.
			return nil, fmt.Errorf("op %d: array parent at %s", i, op.Path)
		default:
			return nil, fmt.Errorf("op %d: parent of %s is not a container", i, op.Path)
		}
	}
	return doc, nil
}

// rfcWalk resolves toks below doc; every token must name an existing member or element.
func rfcWalk(doc any, toks []string) (any, error) {
	cur := doc
	for _, t := range toks {
		switch c := cur.(type) {
		case map[string]any:
			v, ok := c[t]
			if !ok {
				return nil, fmt.Errorf("missing member %q", t)
			}
			cur = v
		case []any:
			n, err := strconv.Atoi(t)
			if err != nil || n < 0 || n >= len(c) || (len(t) > 1 && t[0] == '0') {
				return nil, fmt.Errorf("bad index %q", t)
			}
			cur = c[n]
		default:
			return nil, fmt.Errorf("token %q below a scalar", t)
		}
	}
	return cur, nil
}

// TestRFC6902Applier checks the applier against examples of RFC 6902 appendix A and the
// failure rules of 4.2 and 4.3, so the property tests stand on a checked oracle.
func TestRFC6902Applier(t *testing.T) {
	t.Parallel()
	cases := []struct {
		doc  string
		ops  []domain.NormOp
		want string
		fail bool
	}{
		{`{"foo":"bar"}`, []domain.NormOp{{Op: domain.OpAdd, Path: "/baz", Value: json.RawMessage(`"qux"`)}}, `{"baz":"qux","foo":"bar"}`, false},
		{`{"baz":"qux","foo":"bar"}`, []domain.NormOp{{Op: domain.OpRemove, Path: "/baz"}}, `{"foo":"bar"}`, false},
		{`{"baz":"qux","foo":"bar"}`, []domain.NormOp{{Op: domain.OpReplace, Path: "/baz", Value: json.RawMessage(`"boo"`)}}, `{"baz":"boo","foo":"bar"}`, false},
		{`{"foo":{"bar":"baz"}}`, []domain.NormOp{{Op: domain.OpAdd, Path: "/foo/child", Value: json.RawMessage(`{"grandchild":{}}`)}}, `{"foo":{"bar":"baz","child":{"grandchild":{}}}}`, false},
		{`{"a/b":1,"m~n":2}`, []domain.NormOp{{Op: domain.OpRemove, Path: "/a~1b"}, {Op: domain.OpReplace, Path: "/m~0n", Value: json.RawMessage(`3`)}}, `{"m~n":3}`, false},
		{`{"foo":"bar"}`, []domain.NormOp{{Op: domain.OpReplace, Path: "/baz", Value: json.RawMessage(`1`)}}, ``, true},
		{`{"foo":"bar"}`, []domain.NormOp{{Op: domain.OpRemove, Path: "/baz"}}, ``, true},
		{`{"foo":"bar"}`, []domain.NormOp{{Op: domain.OpAdd, Path: "/baz/bat", Value: json.RawMessage(`1`)}}, ``, true},
	}
	for i, c := range cases {
		doc, _ := rfcDecode([]byte(c.doc))
		got, err := rfcApply(doc, c.ops)
		if c.fail {
			if err == nil {
				t.Errorf("case %d: no error", i)
			}
			continue
		}
		if err != nil {
			t.Fatalf("case %d: %v", i, err)
		}
		b, _ := json.Marshal(got)
		if !bytes.Equal(b, []byte(c.want)) {
			t.Errorf("case %d: got %s, want %s", i, b, c.want)
		}
	}
}
