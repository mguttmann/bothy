// SPDX-License-Identifier: Apache-2.0

// Package hostostest is the fake operating system of the tests: users with groups, a file tree
// with owners, modes, ACLs and links, and probes. Every rule of doctor, helper and install runs
// against it on any build host without administrator rights; the platform suite proves the real
// adapters.
package hostostest

import (
	"context"
	"errors"
	"io/fs"
	"path"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/OWNER/bothy/services/host/internal/hostos"
)

// Node is one entry of the fake file tree.
type Node struct {
	Sec hostos.FileSec
	// Target is the target of a link.
	Target string
	// ID overrides the identity (default: the path of the node).
	ID string
	// Content is the content of a file.
	Content []byte
}

// World is a fake host. Paths use forward slashes and start at "/"; tests of Windows rules use the
// pure functions of hostos with Windows SIDs.
type World struct {
	mu       sync.Mutex
	GOOS     string
	Nodes    map[string]*Node
	Accts    []hostos.Account
	GroupMap map[string][]hostos.Group
	SudoOK   map[string]bool
	Ptrace   int
	// ListHook is called on every ReadDir; tests use it to move a fake clock during a scan.
	ListHook func(path string)
}

// New returns a world for goos with the root folder "/" owned by root, mode 0755, and the user
// root (uid 0).
func New(goos string) *World {
	w := &World{GOOS: goos, Nodes: map[string]*Node{}, GroupMap: map[string][]hostos.Group{}, SudoOK: map[string]bool{}, Ptrace: 1}
	w.Nodes["/"] = &Node{Sec: hostos.FileSec{IsDir: true, OwnerID: "0", GroupID: "0", Mode: 0o755}}
	w.AddUser("root", "0", "/root", false)
	return w
}

// AddUser adds an account with its primary group of the same name and ID plus groups.
func (w *World) AddUser(name, id, home string, person bool, groups ...hostos.Group) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.Accts = append(w.Accts, hostos.Account{Name: name, ID: id, Home: home, Person: person, Shell: "/bin/sh"})
	w.GroupMap[name] = append([]hostos.Group{{Name: name, ID: id}}, groups...)
}

// SetGroups replaces the groups of name.
func (w *World) SetGroups(name string, groups ...hostos.Group) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.GroupMap[name] = groups
}

// UID returns the ID of the account name, or "" when it does not exist.
func (w *World) UID(name string) string {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, a := range w.Accts {
		if a.Name == name {
			return a.ID
		}
	}
	return ""
}

func (w *World) mkparents(p string) {
	for d := path.Dir(p); ; d = path.Dir(d) {
		if _, ok := w.Nodes[d]; !ok {
			w.Nodes[d] = &Node{Sec: hostos.FileSec{IsDir: true, OwnerID: "0", GroupID: "0", Mode: 0o755}}
		}
		if d == "/" {
			return
		}
	}
}

// Dir adds a folder (and missing parents owned by root with mode 0755).
func (w *World) Dir(p, owner string, mode fs.FileMode, acl ...hostos.ACE) *Node {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.mkparents(p)
	n := &Node{Sec: hostos.FileSec{IsDir: true, OwnerID: owner, GroupID: owner, Mode: mode, ACL: acl}}
	w.Nodes[p] = n
	return n
}

// File adds a file.
func (w *World) File(p, owner string, mode fs.FileMode, content string, acl ...hostos.ACE) *Node {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.mkparents(p)
	n := &Node{Sec: hostos.FileSec{OwnerID: owner, GroupID: owner, Mode: mode, ACL: acl}, Content: []byte(content)}
	w.Nodes[p] = n
	return n
}

// Link adds a symbolic link owned by owner.
func (w *World) Link(p, owner, target string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.mkparents(p)
	w.Nodes[p] = &Node{Sec: hostos.FileSec{IsLink: true, OwnerID: owner, GroupID: owner, Mode: 0o777}, Target: target}
}

// Remove deletes p and everything below it.
func (w *World) Remove(p string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for k := range w.Nodes {
		if k == p || strings.HasPrefix(k, p+"/") {
			delete(w.Nodes, k)
		}
	}
}

// Node returns the node at p or nil.
func (w *World) Node(p string) *Node {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.Nodes[p]
}

// Subject returns the evaluator view of the account name.
func (w *World) Subject(name string) hostos.Subject {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.subject(name)
}

func (w *World) subject(name string) hostos.Subject {
	s := hostos.Subject{Name: name}
	for _, a := range w.Accts {
		if a.Name == name {
			s.ID = a.ID
		}
	}
	for _, g := range w.GroupMap[name] {
		s.GroupIDs = append(s.GroupIDs, g.ID)
	}
	return s
}

// Users returns the Users of the world.
func (w *World) Users() hostos.Users { return users{w} }

type users struct{ w *World }

func (u users) Lookup(name string) (hostos.Account, error) {
	u.w.mu.Lock()
	defer u.w.mu.Unlock()
	for _, a := range u.w.Accts {
		if a.Name == name {
			return a, nil
		}
	}
	return hostos.Account{}, hostos.ErrUnknownUser
}

func (u users) Groups(name string) ([]hostos.Group, error) {
	u.w.mu.Lock()
	defer u.w.mu.Unlock()
	g, ok := u.w.GroupMap[name]
	if !ok {
		return nil, hostos.ErrUnknownUser
	}
	return slices.Clone(g), nil
}

func (u users) Accounts() ([]hostos.Account, error) {
	u.w.mu.Lock()
	defer u.w.mu.Unlock()
	return slices.Clone(u.w.Accts), nil
}

// Security returns the Security of the world.
func (w *World) Security() hostos.Security { return security{w} }

type security struct{ w *World }

// ErrNotExist is the error of a missing node.
var ErrNotExist = fs.ErrNotExist

func (s security) Lstat(p string) (hostos.FileSec, error) {
	s.w.mu.Lock()
	defer s.w.mu.Unlock()
	n, ok := s.w.Nodes[path.Clean(p)]
	if !ok {
		return hostos.FileSec{}, ErrNotExist
	}
	return n.Sec, nil
}

// As returns the view of the world with the rights of the account name: its Resolver and Lister
// see only what that account can traverse and list.
func (w *World) As(name string) View { return View{w: w, user: name} }

// View is the world seen by one account.
type View struct {
	w    *World
	user string
}

// traversable reports whether every folder above p can be passed by the user.
func (v View) traversable(p string) bool {
	s := v.w.subject(v.user)
	for d := path.Dir(p); ; d = path.Dir(d) {
		n, ok := v.w.Nodes[d]
		if !ok || !hostos.Readable(n.Sec, s) {
			return false
		}
		if d == "/" {
			return true
		}
	}
}

// IsLink implements hostos.LinkFS.
func (v View) IsLink(p string) (bool, error) {
	v.w.mu.Lock()
	defer v.w.mu.Unlock()
	n, ok := v.w.Nodes[p]
	if !ok || !v.traversable(p) {
		return false, ErrNotExist
	}
	return n.Sec.IsLink, nil
}

// Readlink implements hostos.LinkFS.
func (v View) Readlink(p string) (string, error) {
	v.w.mu.Lock()
	defer v.w.mu.Unlock()
	n, ok := v.w.Nodes[p]
	if !ok || !n.Sec.IsLink {
		return "", ErrNotExist
	}
	return n.Target, nil
}

// Resolve implements hostos.Resolver.
func (v View) Resolve(p string) (hostos.Resolved, error) {
	return hostos.Resolve(hostos.LinkFuncs{IsLinkFunc: v.IsLink, ReadlinkFunc: v.Readlink}, p)
}

// Identity implements hostos.Resolver: the ID of the node after resolution.
func (v View) Identity(p string) (string, error) {
	r, err := v.Resolve(p)
	if err != nil {
		return "", err
	}
	v.w.mu.Lock()
	defer v.w.mu.Unlock()
	n := v.w.Nodes[r.Final]
	if n.ID != "" {
		return n.ID, nil
	}
	return r.Final, nil
}

// ReadDir implements hostos.Lister: the names of the children when the user can traverse to p and
// read it.
func (v View) ReadDir(p string) ([]string, error) {
	if h := v.w.ListHook; h != nil {
		h(p)
	}
	v.w.mu.Lock()
	defer v.w.mu.Unlock()
	n, ok := v.w.Nodes[p]
	if !ok || !n.Sec.IsDir || !v.traversable(p) || !hostos.Readable(n.Sec, v.w.subject(v.user)) {
		return nil, fs.ErrPermission
	}
	var out []string
	for k := range v.w.Nodes {
		if k != p && path.Dir(k) == p {
			out = append(out, path.Base(k))
		}
	}
	sort.Strings(out)
	return out, nil
}

// Head implements hostos.Lister: the start of the content of p when the user can read it.
func (v View) Head(p string, n int) ([]byte, error) {
	b, err := v.readFile(p)
	if len(b) > n {
		b = b[:n]
	}
	return b, err
}

func (v View) readFile(p string) ([]byte, error) {
	v.w.mu.Lock()
	defer v.w.mu.Unlock()
	n, ok := v.w.Nodes[p]
	if !ok || !v.traversable(p) || !hostos.Readable(n.Sec, v.w.subject(v.user)) {
		return nil, fs.ErrPermission
	}
	return slices.Clone(n.Content), nil
}

// Probe returns the Probe of the account name.
func (v View) Probe() hostos.Probe { return probe(v) }

type probe View

func (p probe) SudoWithoutPassword(context.Context) (bool, error) {
	p.w.mu.Lock()
	defer p.w.mu.Unlock()
	return p.w.SudoOK[p.user], nil
}

func (p probe) PtraceScope() (int, error) {
	p.w.mu.Lock()
	defer p.w.mu.Unlock()
	if p.w.GOOS != "linux" {
		return -1, nil
	}
	return p.w.Ptrace, nil
}

// Runner is a fake hostos.Runner: it records every command and answers with the function Answer.
type Runner struct {
	mu     sync.Mutex
	Calls  []hostos.Cmd
	Answer func(hostos.Cmd) (hostos.Output, error)
}

// Run implements hostos.Runner.
func (r *Runner) Run(_ context.Context, c hostos.Cmd) (hostos.Output, error) {
	r.mu.Lock()
	r.Calls = append(r.Calls, c)
	answer := r.Answer
	r.mu.Unlock()
	if answer == nil {
		return hostos.Output{}, nil
	}
	return answer(c)
}

// Recorded returns a copy of the recorded commands.
func (r *Runner) Recorded() []hostos.Cmd {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.Calls)
}

// ErrFake is a generic injected failure.
var ErrFake = errors.New("hostostest: injected failure")
