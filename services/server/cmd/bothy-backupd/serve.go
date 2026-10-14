// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// defaultStanza is the pgBackRest stanza of a Bothy instance; the entry point of the image writes
// the same name into pgbackrest.conf and archive_command (services/server/images/bothy-postgres).
const defaultStanza = "bothy"

// defaultRetry is the default of --retry: the wait between two attempts of stanza-create while
// the database or its pgBackRest server is not up yet.
const defaultRetry = 5 * time.Second

// serveOptions are the flags of the default command.
type serveOptions struct {
	// pgbackrest is the binary, a path or a name looked up in PATH.
	pgbackrest string
	// stanza is the stanza created on start.
	stanza string
	// retry is the wait between two attempts of stanza-create.
	retry time.Duration
}

// serve starts "pgbackrest server" as a child process with this process's stdout and stderr
// going to stderr, creates the stanza in the background, and waits. On a signal from signals it
// sends SIGTERM to the child, waits for it and returns 0; when ctx ends it does the same. When the
// child exits on its own the repository host is gone, so serve returns 1 and the container's
// restart policy starts it again.
//
// Stanza creation repeats "pgbackrest --stanza=<stanza> stanza-create" every opts.retry until it
// succeeds; pgBackRest completes it without a change when the stanza exists and is valid, so
// every start runs it. A failed attempt is logged with pgBackRest's own message.
func serve(ctx context.Context, opts serveOptions, signals <-chan os.Signal, stderr io.Writer) int {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	log := &lineWriter{w: stderr}

	// The child outlives a canceled ctx until stop has sent SIGTERM: exec would kill it outright.
	child := exec.CommandContext(context.WithoutCancel(ctx), opts.pgbackrest, "server") //nolint:gosec // G204: the binary is a flag of the operator.
	child.Stdout, child.Stderr = log, log
	if err := child.Start(); err != nil {
		log.printf("bothy-backupd: start the pgBackRest TLS server: %v", err)
		return exitFailure
	}
	exited := make(chan error, 1)
	go func() { exited <- child.Wait() }()

	stanzaDone := make(chan struct{})
	go func() {
		defer close(stanzaDone)
		createStanza(ctx, opts, log)
	}()

	var code int
	select {
	case err := <-exited:
		log.printf("bothy-backupd: the pgBackRest TLS server exited: %v", errorText(err))
		code = exitFailure
	case sig := <-signals:
		log.printf("bothy-backupd: %v, stopping the pgBackRest TLS server", sig)
		code = stop(child, exited, log)
	case <-ctx.Done():
		code = stop(child, exited, log)
	}
	cancel()
	<-stanzaDone
	return code
}

// stop sends SIGTERM to the running child and waits until it has exited. The container runtime
// bounds the wait with its own stop timeout, so there is no second one here.
func stop(child *exec.Cmd, exited <-chan error, log *lineWriter) int {
	if err := child.Process.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
		log.printf("bothy-backupd: signal the pgBackRest TLS server: %v", err)
	}
	if err := <-exited; err != nil {
		var exit *exec.ExitError
		// Ended by the SIGTERM itself is the expected outcome.
		if !errors.As(err, &exit) || !exitedBySIGTERM(exit) {
			log.printf("bothy-backupd: the pgBackRest TLS server stopped with %v", errorText(err))
		}
	}
	return exitOK
}

// exitedBySIGTERM reports whether the process ended through SIGTERM.
func exitedBySIGTERM(exit *exec.ExitError) bool {
	ws, ok := exit.Sys().(syscall.WaitStatus)
	return ok && ws.Signaled() && ws.Signal() == syscall.SIGTERM
}

// createStanza runs stanza-create until it succeeds or ctx ends.
func createStanza(ctx context.Context, opts serveOptions, log *lineWriter) {
	for {
		cmd := exec.CommandContext(ctx, opts.pgbackrest, "--stanza="+opts.stanza, "stanza-create") //nolint:gosec // G204: binary and stanza are flags of the operator.
		cmd.Stdout, cmd.Stderr = log, log
		err := cmd.Run()
		if err == nil {
			log.printf("bothy-backupd: stanza %s is ready", opts.stanza)
			return
		}
		if ctx.Err() != nil {
			return
		}
		log.printf("bothy-backupd: stanza-create failed (%v), next attempt in %v", errorText(err), opts.retry)
		select {
		case <-ctx.Done():
			return
		case <-time.After(opts.retry):
		}
	}
}

// errorText renders a wait error; nil means a clean exit with code 0.
func errorText(err error) string {
	if err == nil {
		return "exit status 0"
	}
	return err.Error()
}

// lineWriter serializes writes of the child processes and of serve to one writer, so lines of
// concurrent writers do not interleave within a write.
type lineWriter struct {
	mu sync.Mutex
	w  io.Writer
}

// Write writes p as one unit; it implements io.Writer.
func (l *lineWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}

// printf writes one formatted line.
func (l *lineWriter) printf(format string, args ...any) {
	_, _ = fmt.Fprintf(l, format+"\n", args...)
}
