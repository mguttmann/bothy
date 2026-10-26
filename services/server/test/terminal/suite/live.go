// SPDX-License-Identifier: Apache-2.0

package suite

import (
	"testing"

	"github.com/OWNER/bothy/services/server/test/harness"
	"github.com/OWNER/bothy/services/server/test/terminal/stack"
)

// Live is a world for a Go test of an e2e package that is no scenario file (the tests of
// requirements 21, 23 and 26): the same world, steps and checks as a case, driven from Go.
type Live struct {
	w *instance
}

// NewLive builds a world named name from world for t; hosts limits the hosts it starts (none:
// all).
func NewLive(t *testing.T, env *Env, name string, world World, hosts ...string) *Live {
	t.Helper()
	sc := &Scenario{ID: name, Profile: env.ProfileName, World: world}
	return &Live{w: newInstance(t, env, sc, Case{Name: name, Hosts: hosts})}
}

// Do runs the steps in order and fails the test at the first mismatch.
func (l *Live) Do(steps ...Step) {
	l.w.t.Helper()
	for i, s := range steps {
		if msgs := l.w.step(i, s); len(msgs) > 0 {
			l.w.t.Fatal(Failure(l.w.sc.ID, l.w.c.Name, i, msgs))
		}
	}
}

// Host returns the host container of a host of the world.
func (l *Live) Host(name string) *stack.Host {
	l.w.t.Helper()
	h, err := l.w.container(name)
	if err != nil {
		l.w.t.Fatal(err)
	}
	return h
}

// HostID returns the ID of a host of the world.
func (l *Live) HostID(name string) string { return l.w.hostID(name) }

// Owner returns the client of the case's Owner of the world.
func (l *Live) Owner() *harness.Client { return l.w.owner() }

// Hosts returns the names of the hosts the world started.
func (l *Live) Hosts() []string {
	l.w.mu.Lock()
	defer l.w.mu.Unlock()
	out := make([]string, 0, len(l.w.hosts))
	for n := range l.w.hosts {
		out = append(out, n)
	}
	return out
}

// Expand resolves the references of s in the world (${alex.id}, ${marker}, ...).
func (l *Live) Expand(s string) string { return l.w.expand(s) }

// ShellSteps returns the steps that open a shell session named session of device on host and wait
// until a printf with the marker answers: step-up, open, connect, handshake, start, send and
// expect_output. extra is typed into the shell before the printf (for example background
// processes).
func ShellSteps(dev, host, session, extra string) []Step {
	ok := 201
	return []Step{
		{Device: dev, Do: "step_up"},
		{Device: dev, Do: "open", On: host, Session: session, Expect: &Expect{Status: &ok}},
		{Device: dev, Do: "connect", Session: session, Expect: &Expect{}},
		{Device: dev, Do: "handshake", Session: session, Expect: &Expect{}},
		{Device: dev, Do: "start", Session: session},
		{Device: dev, Do: "send", Session: session, Text: extra + "printf '%s-%s\\n' ready ${marker}\n"},
		{Device: dev, Do: "expect_output", Session: session, Expect: &Expect{Output: "ready-${marker}"}},
	}
}

// Person returns the client of the first device of a person of the world (M4-36: the client
// stack enrolls further devices for the app's test).
func (l *Live) Person(name string) *harness.Client {
	l.w.t.Helper()
	p, ok := l.w.people[name]
	if !ok {
		l.w.t.Fatalf("%s: unknown person %s", l.w.sc.ID, name)
	}
	return p.client
}

// GrantID returns the ID of a named grant of the world.
func (l *Live) GrantID(name string) string {
	l.w.t.Helper()
	id, ok := l.w.grants[name]
	if !ok {
		l.w.t.Fatalf("%s: unknown grant %s", l.w.sc.ID, name)
	}
	return id
}
