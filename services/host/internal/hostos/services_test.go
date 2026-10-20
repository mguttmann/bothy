// SPDX-License-Identifier: Apache-2.0

package hostos

import (
	"context"
	"errors"
	"testing"
)

type answer struct {
	out Output
	err error
	got []Cmd
}

func (a *answer) Run(_ context.Context, c Cmd) (Output, error) {
	a.got = append(a.got, c)
	return a.out, a.err
}

func TestServiceNames(t *testing.T) {
	for _, tc := range [][3]string{
		{"linux", HelperServiceName("linux", "a"), "bothy-host-helper-a.service"},
		{"darwin", HelperServiceName("darwin", "a"), "bothy-host.helper.a"},
		{"windows", HelperServiceName("windows", "a"), "bothy-host-helper-a"},
		{"linux", HostServiceName("linux"), "bothy-host.service"},
		{"darwin", HostServiceName("darwin"), "bothy-host"},
	} {
		if tc[1] != tc[2] {
			t.Errorf("%s: %q", tc[0], tc[1])
		}
	}
}

func TestMainPID(t *testing.T) {
	a := &answer{out: Output{Stdout: []byte("4242\n")}}
	if pid, err := (Systemd{a}).MainPID(context.Background(), "x.service"); err != nil || pid != 4242 {
		t.Errorf("%d %v", pid, err)
	}
	if a.got[0].Path != "/usr/bin/systemctl" || len(a.got[0].Args) != 4 {
		t.Errorf("%+v", a.got[0])
	}
	for _, out := range []Output{{Stdout: []byte("0\n")}, {ExitCode: 1}} {
		if _, err := (Systemd{&answer{out: out}}).MainPID(context.Background(), "x"); !errors.Is(err, ErrNotRunning) {
			t.Errorf("%+v: %v", out, err)
		}
	}
	l := &answer{out: Output{Stdout: []byte("system/bothy-host.helper.a = {\n\tstate = running\n\tpid = 731\n}\n")}}
	if pid, err := (Launchd{l}).MainPID(context.Background(), "bothy-host.helper.a"); err != nil || pid != 731 || l.got[0].Args[1] != "system/bothy-host.helper.a" {
		t.Errorf("%d %v", pid, err)
	}
	if _, err := (Launchd{&answer{out: Output{Stdout: []byte("state = waiting\n")}}}).MainPID(context.Background(), "x"); !errors.Is(err, ErrNotRunning) {
		t.Error("no pid line")
	}
	if _, err := (Launchd{&answer{err: errors.New("x")}}).MainPID(context.Background(), "x"); !errors.Is(err, ErrNotRunning) {
		t.Error("runner error")
	}
}
