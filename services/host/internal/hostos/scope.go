// SPDX-License-Identifier: Apache-2.0

package hostos

import (
	"context"
	"sync"
)

// Scope binds users and sec to one run of the checks (doctor.RunService). Every program they
// start runs under ctx as well, so the run ends when ctx ends and starts nothing after that; and
// each answer of the user database (an account, the account list, the groups of an account, the
// ID of a group) is read at most once per run. On macOS every lookup otherwise starts dscacheutil
// and every group of an ACL dscl, which made one run start dozens of them (the gate before a
// PTY took longer than the opening window on a loaded host). A run reads its own fresh answers:
// the cache lives only as long as the returned values.
func Scope(ctx context.Context, users Users, sec Security) (Users, Security) {
	u := &runUsers{inner: users, lookups: newOnceMap[Account](), groups: newOnceMap[[]Group]()}
	if b, ok := users.(usersInRun); ok {
		u.inner, u.fromAccounts = b.inRun(ctx)
	}
	if b, ok := sec.(securityInRun); ok {
		sec = b.inRun(ctx, u)
	}
	return u, sec
}

// usersInRun is a Users that starts programs: inRun returns it with its programs bound to ctx
// and tells whether its Lookup is the search of a name in its Accounts.
type usersInRun interface {
	inRun(ctx context.Context) (Users, bool)
}

// securityInRun is a Security that starts programs and looks up users: inRun returns it with its
// programs bound to ctx, its lookups answered by users and its group IDs cached.
type securityInRun interface {
	inRun(ctx context.Context, users Users) Security
}

// runUsers caches the answers of a Users for one run.
type runUsers struct {
	inner        Users
	fromAccounts bool
	accounts     onceValue[[]Account]
	lookups      *onceMap[Account]
	groups       *onceMap[[]Group]
}

// Lookup implements Users.
func (u *runUsers) Lookup(name string) (Account, error) {
	if u.fromAccounts {
		accs, err := u.Accounts()
		if err != nil {
			return Account{}, err
		}
		return lookupIn(accs, name)
	}
	return u.lookups.get(name, u.inner.Lookup)
}

// Groups implements Users.
func (u *runUsers) Groups(name string) ([]Group, error) { return u.groups.get(name, u.inner.Groups) }

// Accounts implements Users.
func (u *runUsers) Accounts() ([]Account, error) { return u.accounts.get(u.inner.Accounts) }

// onceValue is a value read at most once.
type onceValue[T any] struct {
	mu   sync.Mutex
	done bool
	v    T
	err  error
}

func (o *onceValue[T]) get(read func() (T, error)) (T, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.done {
		o.v, o.err = read()
		o.done = true
	}
	return o.v, o.err
}

// onceMap is a value per name, each read at most once.
type onceMap[T any] struct {
	mu sync.Mutex
	m  map[string]*onceValue[T]
}

func newOnceMap[T any]() *onceMap[T] { return &onceMap[T]{m: map[string]*onceValue[T]{}} }

func (o *onceMap[T]) get(name string, read func(string) (T, error)) (T, error) {
	o.mu.Lock()
	v := o.m[name]
	if v == nil {
		v = &onceValue[T]{}
		o.m[name] = v
	}
	o.mu.Unlock()
	return v.get(func() (T, error) { return read(name) })
}

// lookupIn returns the account name of accounts or an error wrapping ErrUnknownUser.
func lookupIn(accounts []Account, name string) (Account, error) {
	for _, a := range accounts {
		if a.Name == name {
			return a, nil
		}
	}
	return Account{}, ErrUnknownUser
}
