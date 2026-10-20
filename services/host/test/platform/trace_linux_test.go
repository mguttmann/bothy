// SPDX-License-Identifier: Apache-2.0

//go:build platform && linux

package platform

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
)

// traceService attaches strace to the host service (Design note 17: the file opens of the service
// user and, with read and write, the bytes of its local channels) and returns the stop, which
// returns the trace. The workflow installs strace on the Linux runner; a missing strace fails.
func traceService(t *testing.T) (func() string, bool) {
	t.Helper()
	pid := mainPID(t)
	if pid == 0 {
		t.Fatal("host service not running")
	}
	out := filepath.Join(t.TempDir(), "service.trace")
	cmd := exec.Command("/usr/bin/strace", "-f", "-qq", "-s", "65535", "-o", out, //nolint:gosec // the trace of the suite
		"-e", "trace=open,openat,openat2,read,write,readv,writev,sendto,recvfrom,sendmsg,recvmsg", "-p", strconv.Itoa(pid))
	if err := cmd.Start(); err != nil {
		t.Fatalf("strace: %v", err)
	}
	for i := 0; i < 100; i++ {
		if _, err := os.Stat(out); err == nil {
			break
		}
		time.Sleep(pollEvery)
	}
	time.Sleep(time.Second) // strace attaches to every thread after it created the file
	stopped := false
	stop := func() string {
		if !stopped {
			stopped = true
			_ = cmd.Process.Signal(syscall.SIGINT)
			_ = cmd.Wait()
		}
		b, _ := os.ReadFile(out)
		return string(b)
	}
	t.Cleanup(func() { stop() })
	return stop, true
}
