// SPDX-License-Identifier: Apache-2.0

// Package startup is the start sequence of "bothy-host run" (docs/spec/24-terminal.md 2.2,
// docs/DECISIONS.md D-M2-42-15): load the configuration, open the state and the admin endpoint,
// run doctor in mode Full, log in through the Control port, apply the declaration of ready,
// report, and refuse the start with exit code 78 on a "start refused" finding. It also serves the
// op doctor.run of the admin endpoint, tells M2-45 the readiness per session user, and adapts the
// local channel to the Helpers of doctor.
package startup

import (
	"context"
	"errors"

	"github.com/OWNER/bothy/services/host/internal/doctor"
)

// ExitStartRefused is the exit code of a refused start: EX_CONFIG of sysexits. No service
// definition of install restarts the host service after it (D-M2-42-2, -14).
const ExitStartRefused = 78

// Ready is what the host service learns from the message ready of the control connection.
type Ready struct {
	// LLMCredentials is hosts.llm_credentials: none, personal or organization.
	LLMCredentials string
}

// Report is the message report (HostReport): codes and names only.
type Report struct {
	Profiles            []doctor.Profile             `json:"profiles"`
	SessionUsers        []doctor.ReportedSessionUser `json:"session_users"`
	Doctor              []doctor.ReportEntry         `json:"doctor"`
	IsServerHost        bool                         `json:"is_server_host"`
	RequireLocalConfirm bool                         `json:"require_local_confirm"`
	OSVersion           string                       `json:"os_version,omitempty"`
}

// Control is the port of the control connection; M2-43 implements it over /hosts/connect.
type Control interface {
	// Login opens the control connection and returns the content of ready.
	Login(ctx context.Context) (Ready, error)
	// Report sends one report.
	Report(ctx context.Context, r Report) error
	// Close closes the control connection.
	Close() error
}

// Events is the optional part of a Control that keeps its connection after the login (M2-43):
// every later ready reaches the handler, and Stopped closes when the connection stopped for good
// (close codes 4110, 4112, 4114), after which run exits like a refused start.
type Events interface {
	// SetReadyHandler sets the function every later ready reaches.
	SetReadyHandler(f func(context.Context, Ready))
	// Stopped is closed when the connection stopped for good.
	Stopped() <-chan struct{}
}

// ErrUnreachable is the answer of NoControl: the host cannot reach the server (24 2.2).
var ErrUnreachable = errors.New("startup: server unreachable")

// NoControl is the Control of this ticket: every Login fails with ErrUnreachable, so run behaves
// like a host that cannot reach the server until M2-43 replaces the wiring.
type NoControl struct{}

// Login implements Control.
func (NoControl) Login(context.Context) (Ready, error) { return Ready{}, ErrUnreachable }

// Report implements Control.
func (NoControl) Report(context.Context, Report) error { return ErrUnreachable }

// Close implements Control.
func (NoControl) Close() error { return nil }
