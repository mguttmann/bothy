// SPDX-License-Identifier: Apache-2.0

package hostos

import (
	"errors"
	"reflect"
	"testing"
)

type mapFS map[string]string // path -> link target ("" for a plain entry); missing means absent

func (m mapFS) IsLink(p string) (bool, error) {
	t, ok := m[p]
	if !ok {
		return false, errors.New("absent")
	}
	return t != "", nil
}

func (m mapFS) Readlink(p string) (string, error) {
	if t := m[p]; t != "" {
		return t, nil
	}
	return "", errors.New("no link")
}

func TestResolve(t *testing.T) {
	fsys := mapFS{"/a": "", "/a/b": "../c", "/c": "", "/c/d": "/e/f", "/e": "", "/e/f": "", "/e/f/g": "", "/loop": "/loop", "/dangling": "/nowhere"}
	r, err := Resolve(fsys, "/a/b/d/./g")
	if err != nil {
		t.Fatal(err)
	}
	want := Resolved{
		Final:      "/e/f/g",
		Chain:      []string{"/", "/e", "/e/f"},
		Components: []string{"/a", "/a/b", "/a/b/d", "/a/b/d/g"},
		Links:      []Link{{Path: "/a/b", Dir: "/a"}, {Path: "/c/d", Dir: "/c"}},
	}
	if !reflect.DeepEqual(r, want) {
		t.Errorf("got %+v\nwant %+v", r, want)
	}
	for _, p := range []string{"relative", "/missing", "/loop", "/dangling", "/a/../missing"} {
		if _, err := Resolve(fsys, p); !errors.Is(err, ErrUnresolvable) {
			t.Errorf("%s: %v", p, err)
		}
	}
	r, err = Resolve(fsys, "/")
	if err != nil || r.Final != "/" || len(r.Chain) != 1 {
		t.Errorf("root: %+v %v", r, err)
	}
}
