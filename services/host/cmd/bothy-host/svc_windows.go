// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"io"
	"slices"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"

	"github.com/OWNER/bothy/services/host/internal/localchan"
	"github.com/OWNER/bothy/services/host/internal/startup"
)

// handler runs the host service under the service control manager. A refused start stops with
// SERVICE_STOPPED, ERROR_SERVICE_SPECIFIC_ERROR and the service-specific code 78, which the
// failure actions do not restart (FailureActionsOnNonCrashFailures false, D-M2-42-14).
type handler struct {
	run func(ctx context.Context, once bool) int
}

func (h handler) Execute(args []string, req <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	status <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
	done := make(chan int, 1)
	go func() { done <- h.run(ctx, slices.Contains(args, "--doctor-once")) }()
	var stopping <-chan time.Time
	for {
		select {
		case code := <-done:
			if code != 0 {
				return true, uint32(code) //nolint:gosec // small exit codes
			}
			return false, 0
		case <-stopping:
			// The stop did not finish within stopGrace: the service reports itself stopped and the
			// process ends, which ends whatever still waits (AUD-PKG-R2-06/9).
			_, _ = io.WriteString(serviceStderr(io.Discard), "bothy-host: the stop did not finish in time; the process ends\n")
			return false, 0
		case c := <-req:
			if c.Cmd == svc.Interrogate {
				status <- c.CurrentStatus
			}
			if (c.Cmd == svc.Stop || c.Cmd == svc.Shutdown) && stopping == nil {
				status <- svc.Status{State: svc.StopPending, WaitHint: uint32(stopGrace / time.Millisecond)}
				cancel()
				stopping = time.After(stopGrace)
			}
		}
	}
}

// stopGrace bounds a stop of a Windows service of bothy-host: after it the service reports
// SERVICE_STOPPED and the process ends even if a part still waits. A PTY helper stayed in
// STOP_PENDING for the rest of every platform run, so its account and profile could not be
// deleted and the next install failed with "The specified service has been marked for deletion"
// (run 45854064951). A protocol constant of the service, not a product window.
const stopGrace = 10 * time.Second

// serviceMode runs run under the service control manager when the process is a Windows service.
func serviceMode(run func(ctx context.Context, once bool) int) (int, bool) {
	is, err := svc.IsWindowsService()
	if err != nil || !is {
		return 0, false
	}
	if err := svc.Run("bothy-host", handler{run: run}); err != nil {
		return exitFailed, true
	}
	return exitOK, true
}

// doctorOnceWindows starts the stopped service with the start argument doctor-once and reads
// its full run from the admin pipe (D-M2-42-1).
func (a *app) doctorOnceWindows(ctx context.Context, stdout, stderr io.Writer) int {
	m, err := mgr.Connect()
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "bothy-host doctor: service control manager unavailable")
		return exitFailed
	}
	defer func() { _ = m.Disconnect() }()
	s, err := m.OpenService("bothy-host")
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "bothy-host doctor: the host service is not installed")
		return exitFailed
	}
	defer func() { _ = s.Close() }()
	if err := s.Start("--doctor-once"); err != nil {
		_, _ = fmt.Fprintln(stderr, "bothy-host doctor: the host service did not start")
		return exitFailed
	}
	users, _, _ := a.platform(a.runner)
	acc, err := users.Lookup(a.serviceAccount())
	if err != nil {
		return exitFailed
	}
	for range 600 {
		if conn, err := localchan.DialAdmin(ctx, "", acc.ID); err == nil {
			ans, err := startup.AskDoctor(ctx, conn)
			_ = conn.Close()
			if err == nil {
				_, _ = fmt.Fprint(stdout, ans.Output)
				return ans.Exit
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	_, _ = fmt.Fprintln(stderr, "bothy-host doctor: no answer of the host service")
	return exitFailed
}

// eventSource is the event log source of the lines a Windows service of bothy-host writes to its
// standard error, which the service control manager discards.
const eventSource = "bothy-host"

// serviceStderr returns w plus the Application event log: a PTY helper runs as a Windows service
// without a console and without the log file of the host service (its account may not write the
// state directory), so the reason it stopped was lost (the open minor of AUD-host-01; it hid
// why every helper ended in the platform runs, AUD-PKG-R2-06/8). The lines carry codes and error
// texts of the system, no content.
func serviceStderr(w io.Writer) io.Writer { return eventWriter{w: w} }

type eventWriter struct{ w io.Writer }

func (e eventWriter) Write(p []byte) (int, error) {
	if l, err := eventlog.Open(eventSource); err == nil {
		_ = l.Error(1, string(p))
		_ = l.Close()
	}
	return e.w.Write(p)
}
