// SPDX-License-Identifier: Apache-2.0

// Package clientstack serves the terminal stack of M2-48 to the stack test of the desktop app
// (brief M4-36 Design note 6; make terminal-stack). TestClientStack runs only when
// BOTHY_CLIENTSTACK_OUT names a file: it starts the stack of the profile default with one host
// container, builds a world with Alex (an own session user on the host, assigned to Alex, and a
// grant terminal), creates an enrollment code for a device of Alex and one for a device of the
// case's Owner (only the newest code of a person is valid, 08 3.2, so the Dart test lets the
// Owner's device create the code of Alex's second device), writes the connection data as JSON to that file and waits until the process
// gets SIGINT or SIGTERM or the file BOTHY_CLIENTSTACK_OUT.stop appears; then the stack is
// removed. Without the variable the test is skipped, so the Go tests of make check never start
// a stack here.
//
// The JSON holds only test data: the instance origin, the published port of the TLS front on
// 127.0.0.1, the certificate (public part) of the test CA, the host ID, the grant ID, the IDs of
// Alex and the Owner and the two one-time enrollment codes.
package clientstack

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/test/harness"
	"github.com/OWNER/bothy/services/server/test/terminal/stack"
	"github.com/OWNER/bothy/services/server/test/terminal/suite"
)

// outEnv names the file the connection data goes to.
const outEnv = "BOTHY_CLIENTSTACK_OUT"

// stopPoll is how often the test looks for the stop file (a test constant; the signal ends the
// wait at once).
const stopPoll = time.Second

// Connection is the JSON the Dart stack test reads.
type Connection struct {
	Origin        string `json:"origin"`
	Host          string `json:"instance_host"`
	Port          int    `json:"published_port"`
	CACertificate string `json:"ca_certificate_pem"`
	HostID        string `json:"host_id"`
	GrantID       string `json:"grant_id"`
	AlexID        string `json:"alex_id"`
	OwnerID       string `json:"owner_id"`
	AlexCode      string `json:"alex_enrollment_code"`
	OwnerCode     string `json:"owner_enrollment_code"`
}

func TestClientStack(t *testing.T) {
	out := os.Getenv(outEnv)
	if out == "" {
		t.Skipf("%s is not set: the stack starts only for make terminal-stack", outEnv)
	}
	env := suite.StartEnv(t, "clientstack", "default")
	live := suite.NewLive(t, env, "clientstack", suite.World{
		People: []string{"alex"},
		Hosts: []suite.HostSpec{{
			Name:         "r1",
			AssignedTo:   "alex",
			SessionUsers: []string{"term-alex"},
			Assign:       map[string]string{"alex": "term-alex"},
		}},
		Grants: []suite.GrantSpec{{Name: "g", Subject: "alex", Host: "r1"}},
	})
	owner := live.Owner()
	alex := live.Person("alex")
	code := func(principal string) string {
		t.Helper()
		resp, err := harness.DoWithStepUp(t.Context(), owner, harness.Request{
			Method: http.MethodPost, Path: "/users/" + principal + "/enrollment-codes",
		})
		if err != nil || resp.Status != http.StatusCreated {
			t.Fatalf("createEnrollmentCode for %s: %v %d", principal, err, resp.Status)
		}
		var body struct {
			Code string `json:"code"`
		}
		resp.JSON(t, &body)
		return body.Code
	}
	conn := Connection{
		Origin:        stack.InstanceOrigin,
		Host:          stack.InstanceHost,
		Port:          env.Stack.FrontPort,
		CACertificate: string(env.Stack.CA.PEM()),
		HostID:        live.HostID("r1"),
		GrantID:       live.GrantID("g"),
		AlexID:        alex.PrincipalID,
		OwnerID:       owner.PrincipalID,
		AlexCode:      code(alex.PrincipalID),
		OwnerCode:     code(owner.PrincipalID),
	}
	b, err := json.MarshalIndent(conn, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	// Written under another name and renamed, so a reader never sees half a file.
	if err := os.WriteFile(out+".tmp", b, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(out+".tmp", out); err != nil {
		t.Fatal(err)
	}
	t.Logf("client stack ready: %s", out)

	ctx, stop := signal.NotifyContext(t.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	wait(ctx, out+".stop")
}

// wait returns when ctx ends or the file stopFile exists.
func wait(ctx context.Context, stopFile string) {
	tick := time.NewTicker(stopPoll)
	defer tick.Stop()
	for {
		if _, err := os.Stat(stopFile); err == nil || !errors.Is(err, os.ErrNotExist) {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}
