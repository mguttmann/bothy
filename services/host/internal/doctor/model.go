// SPDX-License-Identifier: Apache-2.0

// Package doctor holds the self-checks HD1 to HD11 of the host service (docs/spec/24-terminal.md
// 2.2): the service-side checks the service user runs, the helper-side checks each PTY helper runs
// as its session user, the result model, the report entries by the rule for names
// (HostDoctorCheck of docs/api/openapi.yaml) and the local output of "bothy-host doctor".
//
// A result carries codes, names of session users, local-only names (users whose home HD9 could
// list, the drive of drive_root_writable) and identities; never a path, a content or a value.
package doctor

import (
	"fmt"
	"slices"
	"strings"
)

// Check names: the check values of HostDoctorCheck, in the order HD1 to HD11.
const (
	HD1  = "hd1_user"
	HD2  = "hd2_groups"
	HD3  = "hd3_sudo"
	HD4  = "hd4_state_dir"
	HD5  = "hd5_server_host"
	HD6  = "hd6_llm_login"
	HD7  = "hd7_profiles"
	HD8  = "hd8_separation"
	HD9  = "hd9_foreign_homes"
	HD10 = "hd10_helper_integrity"
	HD11 = "hd11_shared_locations"
)

// Checks lists the check names in order.
var Checks = []string{HD1, HD2, HD3, HD4, HD5, HD6, HD7, HD8, HD9, HD10, HD11}

// Codes of doctor (docs/spec/24-terminal.md 2.2 "Codes of doctor", D-M2-42-6, D-M2-42-8 to -10).
const (
	CodeSessionUsersEmpty      = "session_users_empty"
	CodeSessionUserDuplicate   = "session_user_duplicate"
	CodeSessionUserIsService   = "session_user_is_service_user"
	CodePrivilegedUser         = "privileged_user"
	CodeUserMissing            = "user_missing"
	CodeSessionUserNameInvalid = "session_user_name_invalid"

	CodeForbiddenGroup       = "forbidden_group"
	CodeDockerSocketWritable = "docker_socket_writable"

	CodePasswordlessSudo = "passwordless_sudo"

	CodeStateDirOwner       = "state_dir_owner"
	CodeStateDirMode        = "state_dir_mode"
	CodeKeyFileUnprotected  = "key_file_unprotected"
	CodeServerHostDetected  = "server_host_detected"
	CodeSubscriptionLogin   = "subscription_login_detected"
	CodeAuthStatusUnknown   = "auth_status_unknown"
	CodeDeclarationUnknown  = "declaration_unknown"
	CodeAuthStatusUnchecked = "auth_status_unchecked"

	CodeWorkdirDropped           = "workdir_dropped"
	CodeClaudeProfileUnavailable = "claude_profile_unavailable"
	CodeProfileDropped           = "profile_dropped"

	CodeStateAccessForSessionUser = "state_access_for_session_user"
	CodeHelperReadsState          = "helper_reads_state"

	CodeForeignHomeListable = "foreign_home_listable"

	CodeHelperUntrusted      = "helper_untrusted"
	CodeClaudeBinUntrusted   = "claude_bin_untrusted"
	CodePeerMismatch         = "peer_mismatch"
	CodeHelperUnreachable    = "helper_unreachable"
	CodePeerCheckUnavailable = "peer_check_unavailable"
	CodePtraceScopeLow       = "ptrace_scope_low"
	CodeHelperUnprotected    = "helper_unprotected"

	CodeHomeUnresolvable         = "home_unresolvable"
	CodeSharedLocation           = "shared_location"
	CodeForeignEntry             = "foreign_entry"
	CodeEntryScanTimeout         = "entry_scan_timeout"
	CodeEntryScanRunning         = "entry_scan_running"
	CodeDriveRootWritable        = "drive_root_writable"
	CodeProgramLocationUntrusted = "program_location_untrusted"
)

// precedence lists the codes of each check from the most important; one entry per check (and per
// named account) carries the first code that applies.
var precedence = map[string][]string{
	HD1:  {CodeSessionUsersEmpty, CodeSessionUserDuplicate, CodeSessionUserIsService, CodePrivilegedUser, CodeUserMissing, CodeSessionUserNameInvalid},
	HD2:  {CodeForbiddenGroup, CodeDockerSocketWritable},
	HD3:  {CodePasswordlessSudo},
	HD4:  {CodeStateDirOwner, CodeStateDirMode, CodeKeyFileUnprotected},
	HD5:  {CodeServerHostDetected},
	HD6:  {CodeSubscriptionLogin, CodeAuthStatusUnknown, CodeDeclarationUnknown, CodeAuthStatusUnchecked},
	HD7:  {CodeWorkdirDropped, CodeClaudeProfileUnavailable, CodeProfileDropped},
	HD8:  {CodeStateAccessForSessionUser, CodeHelperReadsState},
	HD9:  {CodeForeignHomeListable},
	HD10: {CodeHelperUntrusted, CodeClaudeBinUntrusted, CodePeerMismatch, CodeHelperUnreachable, CodePeerCheckUnavailable, CodePtraceScopeLow, CodeHelperUnprotected},
	HD11: {CodeHomeUnresolvable, CodeSharedLocation, CodeForeignEntry, CodeEntryScanTimeout, CodeEntryScanRunning, CodeDriveRootWritable, CodeProgramLocationUntrusted},
}

// rank returns the position of code in the precedence of check (lower is more important).
func rank(check, code string) int {
	if i := slices.Index(precedence[check], code); i >= 0 {
		return i
	}
	return len(precedence[check])
}

// Status of an entry.
type Status int

// Statuses: a warning does not change the exit code of doctor and refuses nothing.
const (
	Warn Status = iota + 1
	Fail
)

// Entry is one finding of a check.
type Entry struct {
	Status Status
	Code   string
	// SessionUser is the account named in the report (rule for names); empty for an unnamed entry.
	SessionUser string
	// Local holds names only the local output shows ("user=<name>", "drive=<letter>:",
	// "item=<display name>"); they never reach the server.
	Local []string
}

// CheckResult is the result of one check: no entries means passed.
type CheckResult struct {
	Check   string
	Entries []Entry
}

// Failed reports whether the check has a failed entry.
func (c CheckResult) Failed() bool {
	return slices.ContainsFunc(c.Entries, func(e Entry) bool { return e.Status == Fail })
}

// Result is the result of one run of HD1 to HD11 together with the lists after HD7.
type Result struct {
	// Checks holds the eleven checks in order.
	Checks []CheckResult
	// Profiles are the profiles after HD7 (HostProfile).
	Profiles []Profile
	// SessionUsers are the configured session users with the display names of their working
	// directories after HD7 (HostReportedSessionUser).
	SessionUsers []ReportedSessionUser
	// IsServerHost is the finding of HD5.
	IsServerHost bool
	// Declaration is the declaration HD6 was evaluated with.
	Declaration string
	// Workdirs are the working directories after HD7 per session user, in the reported order;
	// M2-45 resolves the index of wdi in exactly this list. The paths never leave the host.
	Workdirs map[string][]Workdir

	facts    facts
	requests map[string]Request
	reached  []string
}

// Profile is a profile as HostProfile reports it.
type Profile struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Kind        string `json:"kind"`
}

// ReportedSessionUser is HostReportedSessionUser.
type ReportedSessionUser struct {
	OSUser   string   `json:"os_user"`
	Workdirs []string `json:"workdirs"`
}

// Workdir is a working directory after HD7: its display name and resolved path (host only).
type Workdir struct {
	DisplayName, Path string
}

// Check returns the result of check name.
func (r *Result) Check(name string) CheckResult {
	for _, c := range r.Checks {
		if c.Check == name {
			return c
		}
	}
	return CheckResult{Check: name}
}

func (r *Result) set(c CheckResult) {
	for i := range r.Checks {
		if r.Checks[i].Check == c.Check {
			r.Checks[i] = c
			return
		}
	}
	r.Checks = append(r.Checks, c)
}

// ReportEntry is a HostDoctorCheck item.
type ReportEntry struct {
	Check       string `json:"check"`
	OK          bool   `json:"ok"`
	Code        string `json:"code,omitempty"`
	SessionUser string `json:"session_user,omitempty"`
}

// Report returns the HostDoctorCheck items of r: one item per check, or per named account of a
// check (rule for names, 24 2.2); check and session_user together are unique. A warning is an
// item with ok true and its code.
func (r *Result) Report() []ReportEntry {
	var out []ReportEntry
	for _, name := range Checks {
		c := r.Check(name)
		if len(c.Entries) == 0 {
			out = append(out, ReportEntry{Check: name, OK: true})
			continue
		}
		byUser := map[string]Entry{}
		var users []string
		for _, e := range c.Entries {
			old, seen := byUser[e.SessionUser]
			if !seen {
				users = append(users, e.SessionUser)
			}
			if !seen || e.Status > old.Status || (e.Status == old.Status && rank(name, e.Code) < rank(name, old.Code)) {
				byUser[e.SessionUser] = e
			}
		}
		for _, u := range users {
			e := byUser[u]
			out = append(out, ReportEntry{Check: name, OK: e.Status != Fail, Code: e.Code, SessionUser: u})
		}
	}
	return out
}

// startRefusing are the checks whose failure refuses the start (24 2.2 table); HD6 fails only at
// a known none or organization, HD11 only an unnamed entry (a place for all) refuses.
var startRefusing = []string{HD1, HD2, HD3, HD4, HD6, HD8, HD9, HD10}

// StartRefused reports whether r refuses the start of the host service.
func (r *Result) StartRefused() bool {
	for _, name := range startRefusing {
		if r.Check(name).Failed() {
			return true
		}
	}
	return r.placeForAllFailed()
}

func (r *Result) placeForAllFailed() bool {
	return slices.ContainsFunc(r.Check(HD11).Entries, func(e Entry) bool { return e.Status == Fail && e.SessionUser == "" })
}

// EndsRunningService reports whether a doctor run inside the running service ends it like a
// refused start: a "start refused" finding other than HD6, HD10 and HD11, which have their own
// consequence in operation (24 2.2 "Output on the host").
func (r *Result) EndsRunningService() bool {
	for _, name := range []string{HD1, HD2, HD3, HD4, HD8, HD9} {
		if r.Check(name).Failed() {
			return true
		}
	}
	return false
}

// HostReady reports whether the host may give sessions at all: no finding of HD6, HD10 or an
// unnamed HD11 entry, and no other start-refusing finding.
func (r *Result) HostReady() bool { return !r.StartRefused() }

// SessionUserReady reports whether sessionUser may get sessions: the host is ready and HD11 has
// no failed entry for it (entry_scan_running included).
func (r *Result) SessionUserReady(sessionUser string) bool {
	if !r.HostReady() {
		return false
	}
	return !slices.ContainsFunc(r.Check(HD11).Entries, func(e Entry) bool { return e.Status == Fail && e.SessionUser == sessionUser })
}

// Exit codes of the command doctor.
const (
	ExitPassed = 0
	ExitFailed = 1
)

// ExitCode returns 1 as soon as one check failed; warnings alone give 0.
func (r *Result) ExitCode() int {
	for _, c := range r.Checks {
		if c.Failed() {
			return ExitFailed
		}
	}
	return ExitPassed
}

// writeEntries writes one line per entry of check name, failures first, each by precedence.
func writeEntries(b *strings.Builder, label, name string, entries []Entry) {
	entries = slices.Clone(entries)
	slices.SortStableFunc(entries, func(x, y Entry) int {
		if x.Status != y.Status {
			return int(y.Status) - int(x.Status)
		}
		return rank(name, x.Code) - rank(name, y.Code)
	})
	for _, e := range entries {
		word := "WARN"
		if e.Status == Fail {
			word = "FAIL"
		}
		b.WriteString(label + " " + word + " " + e.Code)
		if e.SessionUser != "" {
			b.WriteString(" session_user=" + e.SessionUser)
		}
		for _, l := range e.Local {
			b.WriteString(" " + l)
		}
		b.WriteString("\n")
	}
}

// Format returns the local output of doctor (D-M2-42-2): one line per check in the order HD1 to
// HD11, "HD<n> ok", "HD<n> WARN <code>" or "HD<n> FAIL <code>" with "session_user=<name>" for a
// named entry and the local names, one line per entry; then the summary "doctor: <failed> failed,
// <warnings> warnings" counting checks, and "declaration: <value>". Never a path or a value.
func (r *Result) Format() string {
	var b strings.Builder
	failed, warnings := 0, 0
	for i, name := range Checks {
		c := r.Check(name)
		label := fmt.Sprintf("HD%d", i+1)
		if len(c.Entries) == 0 {
			b.WriteString(label + " ok\n")
			continue
		}
		if c.Failed() {
			failed++
		} else {
			warnings++
		}
		writeEntries(&b, label, name, c.Entries)
	}
	fmt.Fprintf(&b, "doctor: %d failed, %d warnings\n", failed, warnings)
	b.WriteString("declaration: " + r.Declaration + "\n")
	return b.String()
}
