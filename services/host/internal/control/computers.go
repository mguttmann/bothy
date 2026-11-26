// SPDX-License-Identifier: Apache-2.0

package control

import "context"

// The host control ops of computer sessions (docs/spec/27-computer-use.md 12.3).
const (
	opComputerOpen    = "computer.open"
	opComputerClose   = "computer.close"
	opComputerDisable = "computer.disable"
	opComputerStarted = "computer.started"
	opComputerState   = "computer.state"
	opComputerEnded   = "computer.ended"
)

// ComputerDataPathPrefix is the start of every data_path of computer.open (channel
// hostComputerData, docs/spec/27-computer-use.md 12.2).
const ComputerDataPathPrefix = "/api/v1/hosts/computer-sessions/"

// ComputerSettings are the server values of a computer session (27 12.3 and 18).
type ComputerSettings struct {
	ImageMaxEdge     int64 `json:"image_max_edge"`
	JPEGQuality      int64 `json:"jpeg_quality"`
	SettleMs         int64 `json:"settle_ms"`
	AutoScreenshot   bool  `json:"auto_screenshot"`
	WaitMaxMs        int64 `json:"wait_max_ms"`
	KeyRepeatMax     int64 `json:"key_repeat_max"`
	AuditAppsMax     int64 `json:"audit_apps_max"`
	StatusIntervalMs int64 `json:"status_interval_ms"`
}

// ComputerOpen is computer.open: user_id is the triggering owner (sub of the attach token).
type ComputerOpen struct {
	SessionID string           `json:"session_id"`
	RunID     string           `json:"run_id"`
	AgentID   string           `json:"agent_id"`
	AgentName string           `json:"agent_name"`
	UserID    string           `json:"user_id"`
	DataPath  string           `json:"data_path"`
	ExpiresAt string           `json:"expires_at"`
	Settings  ComputerSettings `json:"settings"`
}

// ComputerClose is computer.close: the server ended the session.
type ComputerClose struct {
	SessionID string `json:"session_id"`
	Reason    string `json:"reason"`
}

// ComputerDisable is computer.disable: the owner switched the enablement off remotely.
type ComputerDisable struct {
	Generation int64 `json:"generation"`
}

// ComputerStarted is computer.started with the state active or paused.
type ComputerStarted struct {
	SessionID string `json:"session_id"`
	State     string `json:"state"`
}

// AppIdentity is the identity of a frontmost app: bundle ID or executable name, display name and
// the signer's team ID or subject (27 13.1).
type AppIdentity struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Signer string `json:"signer"`
}

// ComputerState is computer.state: the state of a running session for the status chip; FrontApp
// is empty before the helper named an app.
type ComputerState struct {
	SessionID   string      `json:"session_id"`
	State       string      `json:"state"`
	Reason      string      `json:"reason"`
	ActionCount int64       `json:"action_count"`
	FrontApp    AppIdentity `json:"front_app"`
}

// ComputerEnded is computer.ended: the end of a session the host service saw.
type ComputerEnded struct {
	SessionID    string           `json:"session_id"`
	CloseReason  string           `json:"close_reason"`
	CloseDetail  string           `json:"close_detail,omitempty"`
	ActionCounts map[string]int64 `json:"action_counts"`
	Apps         []AppIdentity    `json:"apps"`
	PausedMs     int64            `json:"paused_ms"`
}

// ComputerPermissions are the operating system permissions of the helper: granted, denied or
// unknown each.
type ComputerPermissions struct {
	Capture string `json:"capture"`
	Input   string `json:"input"`
}

// ComputerReport is the object computer of report (27 12.3): the helper state; this object, not a
// doctor check, carries it.
type ComputerReport struct {
	Helper        string              `json:"helper"`
	Enabled       bool                `json:"enabled"`
	Generation    int64               `json:"generation"`
	OSAccount     string              `json:"os_account"`
	Permissions   ComputerPermissions `json:"permissions"`
	HelperVersion string              `json:"helper_version"`
	Signature     string              `json:"signature"`
	Locked        bool                `json:"locked"`
}

// Values of ComputerReport.Helper and of a permission.
const (
	HelperConnected = "connected"
	HelperAbsent    = "absent"
	HelperRefused   = "refused"
	PermUnknown     = "unknown"
	// SignatureVerified and SignatureDevelopment are the values of ComputerReport.Signature.
	SignatureVerified    = "verified"
	SignatureDevelopment = "development"
)

// AbsentReport is the computer object of a host without host_service.computer_use or without a
// connected helper; signature is the state of this build (D-CU-03-6).
func AbsentReport(signature string) ComputerReport {
	return ComputerReport{Helper: HelperAbsent, Permissions: ComputerPermissions{Capture: PermUnknown, Input: PermUnknown}, Signature: signature}
}

// ComputerNotifier sends the computer reports of the host service on the control connection.
type ComputerNotifier interface {
	// ComputerStarted sends computer.started.
	ComputerStarted(ctx context.Context, s ComputerStarted) error
	// ComputerState sends computer.state.
	ComputerState(ctx context.Context, s ComputerState) error
	// ComputerEnded sends computer.ended.
	ComputerEnded(ctx context.Context, e ComputerEnded) error
}

// Computers is the computer session manager of the host service (internal/computer).
type Computers interface {
	// Live returns the computer sessions that run or whose end was not reported yet; they join the
	// live_sessions of identify.
	Live() []string
	// Open handles computer.open; n sends the reports of the session.
	Open(ctx context.Context, o ComputerOpen, n ComputerNotifier)
	// Close handles computer.close.
	Close(ctx context.Context, c ComputerClose)
	// Disable handles computer.disable.
	Disable(ctx context.Context, d ComputerDisable)
	// EndAll ends every computer session without a report (4110, 4111, 4112, 4114).
	EndAll(ctx context.Context)
	// Resend sends the computer.ended that could not be reported before over n.
	Resend(ctx context.Context, n ComputerNotifier)
	// Report returns the object computer of the next report.
	Report() ComputerReport
}

// RefuseComputers is the computer manager of a host without host_service.computer_use: nothing
// listens, the report says helper absent, and every computer.open ends at once with open_failed
// and helper_absent (27 4 item 2).
type RefuseComputers struct {
	// Signature is the signature state of this build for the report.
	Signature string
}

// Compile-time check: the computer port.
var _ Computers = RefuseComputers{}

// Live implements Computers: none.
func (RefuseComputers) Live() []string { return nil }

// Open implements Computers: the session ends with open_failed and helper_absent.
func (RefuseComputers) Open(ctx context.Context, o ComputerOpen, n ComputerNotifier) {
	_ = n.ComputerEnded(ctx, ComputerEnded{SessionID: o.SessionID, CloseReason: reasonOpenFailed, CloseDetail: "helper_absent",
		ActionCounts: map[string]int64{}, Apps: []AppIdentity{}})
}

// Close implements Computers: nothing runs.
func (RefuseComputers) Close(context.Context, ComputerClose) {
	// RefuseComputers never opens a session, so there is none to close.
}

// Disable implements Computers: no helper is connected.
func (RefuseComputers) Disable(context.Context, ComputerDisable) {
	// Without host_service.computer_use no helper is connected that could switch off.
}

// EndAll implements Computers: nothing runs.
func (RefuseComputers) EndAll(context.Context) {
	// RefuseComputers never opens a session, so there is none to end.
}

// Resend implements Computers: nothing is pending.
func (RefuseComputers) Resend(context.Context, ComputerNotifier) {
	// RefuseComputers reports its refusals at once and keeps none.
}

// Report implements Computers: the helper is absent.
func (r RefuseComputers) Report() ComputerReport {
	sig := r.Signature
	if sig == "" {
		sig = SignatureDevelopment
	}
	return AbsentReport(sig)
}
