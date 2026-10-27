// SPDX-License-Identifier: Apache-2.0

package main

import (
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/host/internal/config"
	"github.com/OWNER/bothy/services/host/internal/hostos"
	"github.com/OWNER/bothy/services/host/internal/hostos/hostostest"
)

// TestTargetDarwinGroupIDs: on macOS the install target carries the IDs of the existing groups,
// and a failed listing stops install and session-user add instead of risking a taken ID.
func TestTargetDarwinGroupIDs(t *testing.T) {
	newFake(t)
	sys.goos = hostos.Darwin
	listing := hostos.Output{Stdout: []byte("com.apple.access_remote_ae 400\nstaff 20\n")}
	sys.runner = &hostostest.Runner{Answer: func(c hostos.Cmd) (hostos.Output, error) {
		if c.Path == "/usr/bin/dscl" && slices.Contains(c.Args, "/Groups") {
			return listing, nil
		}
		return hostos.Output{}, nil
	}}
	c := &config.Config{StateDir: "/var/lib/bothy-host", Path: "/Library/Application Support/bothy-host/config.yaml"}
	tg, err := sys.target(c, true)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(tg.GroupIDs, []int{400, 20}) {
		t.Errorf("group IDs %v", tg.GroupIDs)
	}
	listing = hostos.Output{ExitCode: 40}
	if _, err := sys.target(c, true); err == nil {
		t.Error("a failed group listing gave a target")
	}
	sys.goos = hostos.Linux
	if tg, err := sys.target(c, true); err != nil || tg.GroupIDs != nil {
		t.Errorf("linux target %v %v", tg.GroupIDs, err)
	}
}
