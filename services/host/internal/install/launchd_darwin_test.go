// SPDX-License-Identifier: Apache-2.0

package install

import (
	"bytes"
	"context"
	"encoding/xml"
	"flag"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/host/internal/hostos"
)

// withLaunchd turns TestReloadOverARunningLaunchdJob on. The test loads a scratch job into the
// launchd domain of the calling account, so it runs only on request:
// go test ./internal/install -run TestReloadOverARunningLaunchdJob -launchd.
var withLaunchd = flag.Bool("launchd", false, "run the test that loads a scratch job into the launchd domain of this account")

// scratchScript is the program of the scratch job. Like the host service, which ends its
// sessions first, it needs a moment to end after SIGTERM; it writes the file named by its first
// argument once the trap is set, and it ends by itself after a minute, so a test that was killed
// leaves no process behind.
const scratchScript = `trap 'sleep 1; exit 0' TERM; : > "$1"; i=0; while [ "$i" -lt 600 ]; do sleep 0.1; i=$((i+1)); done`

// scratchJob returns a launchd definition of label that runs scratchScript with the marker up.
func scratchJob(t *testing.T, label, up string) string {
	t.Helper()
	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n" +
		`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">` + "\n" +
		`<plist version="1.0"><dict><key>Label</key><string>`)
	text := func(s string) {
		if err := xml.EscapeText(&b, []byte(s)); err != nil {
			t.Fatal(err)
		}
	}
	text(label)
	b.WriteString(`</string><key>RunAtLoad</key><true/><key>ProgramArguments</key><array>`)
	for _, arg := range []string{"/bin/sh", "-c", scratchScript, "sh", up} {
		b.WriteString(`<string>`)
		text(arg)
		b.WriteString(`</string>`)
	}
	b.WriteString(`</array></dict></plist>` + "\n")
	return b.String()
}

// TestReloadOverARunningLaunchdJob runs the steps install uses to load a service (reload) twice
// against the real launchd, with the real executor and a scratch job of its own label: the
// second run meets a running job, as "bothy-host install" does over a running installation. It
// must load the job again. Without the wait for the unload the bootstrap of the second run ended
// with exit code 5 and the job stayed unloaded.
func TestReloadOverARunningLaunchdJob(t *testing.T) {
	if !*withLaunchd {
		t.Skip("needs -launchd: the test loads a scratch job into the launchd domain of this account")
	}
	me, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	if me.Uid == "0" {
		t.Skip("root has no launchd domain of an account")
	}
	ops := OSOps{Runner: hostos.ExecRunner{}}
	ctx := context.Background()
	// The domain of the owner account mode; a login without a GUI session has only the user domain.
	domain := "gui/" + me.Uid
	if ops.Run(ctx, []string{launchctl, "print", domain}) != nil {
		domain = "user/" + me.Uid
		if ops.Run(ctx, []string{launchctl, "print", domain}) != nil {
			t.Skip("this account has no launchd domain")
		}
	}
	// A label of this test process alone, never the label of the host service.
	label := fmt.Sprintf("bothy-host-test-reload-%d", os.Getpid())
	target := domain + "/" + label
	dir := t.TempDir()
	up := filepath.Join(dir, "up")
	plist := filepath.Join(dir, label+".plist")
	if err := os.WriteFile(plist, []byte(scratchJob(t, label, up)), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = ops.Run(ctx, []string{launchctl, "bootout", target})
		for range 100 {
			if ops.Run(ctx, []string{launchctl, "print", target}) != nil {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
		t.Errorf("the scratch job %s is still loaded", target)
	})
	steps := reload(domain, label, plist)
	if err := Apply(ctx, ops, steps); err != nil {
		t.Fatalf("first load: %v", err)
	}
	running := false
	for range 100 {
		if _, err := os.Stat(up); err == nil {
			running = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !running {
		t.Fatal("the scratch job did not start")
	}
	if err := Apply(ctx, ops, steps); err != nil {
		t.Fatalf("load over the running job: %v", err)
	}
	if err := ops.Run(ctx, []string{launchctl, "print", target}); err != nil {
		t.Errorf("the job is not loaded after the second load: %v", err)
	}
}
