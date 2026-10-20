// SPDX-License-Identifier: Apache-2.0

package hostos

import (
	"errors"
	"path/filepath"
	"strings"
)

// ErrUnresolvable is returned for a path that is missing, ends in a dangling link, passes a folder
// the resolving user cannot open, or loops (D-M2-42-7). Such a path counts as absent.
var ErrUnresolvable = errors.New("hostos: path cannot be resolved")

// Resolved is the full resolution of a configured path (24 2.2 point 6).
type Resolved struct {
	// Final is the resolved path (realpath semantics).
	Final string
	// Chain lists each folder of the resolved path from the root down, without Final itself.
	Chain []string
	// Components lists each prefix of the path as configured, from the first component to the
	// whole path; each is checked as written.
	Components []string
	// Links lists each link the resolution followed, with the folder it lies in.
	Links []Link
}

// Link is a symbolic link (or a Windows junction) passed during a resolution.
type Link struct {
	// Path is the link itself, Dir the folder it lies in.
	Path, Dir string
}

// LinkFS is the part of the file system Resolve needs.
type LinkFS interface {
	// IsLink reports whether path is a link; an error means the entry cannot be read.
	IsLink(path string) (bool, error)
	// Readlink returns the target of the link at path.
	Readlink(path string) (string, error)
}

// maxLinks bounds the links of one resolution, like SYMLOOP_MAX of POSIX systems.
const maxLinks = 40

// Resolve resolves the absolute path p like realpath on fsys, recording every link it follows
// and the folders of the result. A relative path is unresolvable.
func Resolve(fsys LinkFS, p string) (Resolved, error) {
	if !filepath.IsAbs(p) {
		return Resolved{}, ErrUnresolvable
	}
	root := rootOf(p)
	out := Resolved{Components: prefixes(p)}
	cur := root
	queue := splitPath(p[len(filepath.VolumeName(p)):])
	links := 0
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		switch c {
		case "", ".":
			continue
		case "..":
			cur = parent(cur)
			continue
		}
		next := filepath.Join(cur, c)
		isLink, err := fsys.IsLink(next)
		if err != nil {
			return Resolved{}, ErrUnresolvable
		}
		if !isLink {
			cur = next
			continue
		}
		links++
		if links > maxLinks {
			return Resolved{}, ErrUnresolvable
		}
		target, err := fsys.Readlink(next)
		if err != nil {
			return Resolved{}, ErrUnresolvable
		}
		out.Links = append(out.Links, Link{Path: next, Dir: cur})
		if filepath.IsAbs(target) {
			target = filepath.Clean(target)
			cur = rootOf(target)
			target = target[len(cur):]
		}
		queue = append(splitPath(target), queue...)
	}
	out.Final = cur
	out.Chain = ancestors(cur)
	return out, nil
}

func rootOf(p string) string {
	v := filepath.VolumeName(p)
	return v + string(filepath.Separator)
}

func splitPath(p string) []string {
	return strings.FieldsFunc(p, func(r rune) bool { return r == filepath.Separator || r == '/' })
}

func parent(p string) string {
	d := filepath.Dir(p)
	return d
}

// prefixes returns "/a", "/a/b", ... for "/a/b" as written, the root excluded; "." and empty
// components are skipped, ".." is kept, because a component is checked as configured.
func prefixes(p string) []string {
	root := rootOf(p)
	var out []string
	cur := root
	for _, c := range splitPath(p[len(filepath.VolumeName(p)):]) {
		if c == "." {
			continue
		}
		if cur == root {
			cur += c
		} else {
			cur += string(filepath.Separator) + c
		}
		out = append(out, cur)
	}
	return out
}

// ancestors returns the root and every folder above p, from the root down.
func ancestors(p string) []string {
	root := rootOf(p)
	out := []string{root}
	pre := prefixes(p)
	if len(pre) > 1 {
		out = append(out, pre[:len(pre)-1]...)
	}
	return out
}

// LinkFuncs is a LinkFS made of two functions; fakes hand their methods in as function values.
type LinkFuncs struct {
	IsLinkFunc   func(path string) (bool, error)
	ReadlinkFunc func(path string) (string, error)
}

// IsLink implements LinkFS.
func (f LinkFuncs) IsLink(path string) (bool, error) { return f.IsLinkFunc(path) }

// Readlink implements LinkFS.
func (f LinkFuncs) Readlink(path string) (string, error) { return f.ReadlinkFunc(path) }
