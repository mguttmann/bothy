// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"errors"

	"github.com/OWNER/bothy/services/server/internal/platform/config"
	terminaldomain "github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// terminalConfigKeys returns the keys of the terminal module whose first consumer is M2-39: the
// text limits of hosts and session users (docs/spec/03-data-model.md section 9), the validity
// of a pairing code (24 section 17), and the validity of a session token, which sets the window
// a retiring signing key of the purpose terminal stays usable (19 section 10); and since M2-40 the
// windows of the fresh step-up and of the quick reattach (24 section 17), read by authz; and since
// M2-41 the confirmation of the fingerprint and the bounds of a report of a host; and since M2-45
// the idle and detach windows the host service applies; and since M2-46 the server's own
// limits of a session.
func terminalConfigKeys() []config.Key {
	return []config.Key{
		{
			Name: "host.name_max_chars", Kind: config.KindInt, Default: "100", Min: "1",
			Doc:     "Longest host name in Unicode characters.",
			Chapter: chapter03,
		},
		{
			Name: "host.os_user_max_chars", Kind: config.KindInt, Default: "64", Min: "1",
			Doc:     "Longest name of a session user in characters.",
			Chapter: chapter03,
		},
		{
			Name: "terminal.pairing_code_ttl", Kind: config.KindDuration, Default: defaultQuarterHour, Min: "1s",
			Doc:     "Validity of the pairing code of a host.",
			Chapter: chapter24,
		},
		{
			Name: keyFingerprintConfirmChars, Kind: config.KindInt, Default: "8", Min: "1",
			Doc:     "Characters of the fingerprint the owner types at the confirmation of a host.",
			Chapter: chapter24,
		},
		{
			Name: keyFingerprintMaxAttempts, Kind: config.KindInt, Default: "3", Min: "1",
			Doc:     "Failed confirmations of a host before it falls back to pairing.",
			Chapter: chapter24,
		},
		{
			Name: "terminal.report_label_max_chars", Kind: config.KindInt, Default: "100", Min: "1",
			Doc:     "Longest profile name, display name, working directory label and OS version a host reports, in characters.",
			Chapter: chapter24,
		},
		{
			Name: "terminal.report_max_entries", Kind: config.KindInt, Default: "64", Min: "1",
			Doc:     "Most entries of each list a host reports: profiles, session users, working directories of one session user.",
			Chapter: chapter24,
		},
		{
			Name: "terminal.session_token_ttl", Kind: config.KindDuration, Default: thirtySeconds, Min: "1s",
			Doc:     "Validity of a session token of the terminal.",
			Chapter: chapter24,
		},
		{
			Name: "terminal.step_up_max_age", Kind: config.KindDuration, Default: sixtySeconds, Min: "1s",
			Doc:     "How young the step-up of terminal.open and terminal.attach must be.",
			Chapter: chapter24,
		},
		{
			Name: "terminal.quick_reattach_window", Kind: config.KindDuration, Default: sixtySeconds, Min: "0s",
			Doc:     "Reattaching from the same device needs no new step-up within this time after the detachment.",
			Chapter: chapter24,
		},
		// The control connection of the host services (M2-43, 24 section 3.5).
		{
			Name: "terminal.host_heartbeat_interval", Kind: config.KindDuration, Default: twentySeconds, Min: minDeadline,
			Doc:     "Interval of the heartbeats of the control connection of a host service, announced in hello.",
			Chapter: chapter24,
		},
		{
			Name: "terminal.host_heartbeat_miss_limit", Kind: config.KindInt, Default: "3", Min: "1",
			Doc:     "Missed heartbeats after which the server closes the control connection with 4009.",
			Chapter: chapter24,
		},
		{
			Name: "terminal.host_auth_timeout", Kind: config.KindDuration, Default: defaultTenSeconds, Min: minDeadline,
			Doc:     "Time between hello and identify on the control and data connections of a host service.",
			Chapter: chapter24,
		},
		// The terminal sessions and the relay (M2-44, 24 sections 6.1 and 6.3).
		{
			Name: "terminal.open_timeout", Kind: config.KindDuration, Default: twentySeconds, Min: "1s",
			Doc:     "Deadline until both sides are connected in the relay and the opening has started.",
			Chapter: chapter24,
		},
		{
			Name: "terminal.relay_buffer_max_bytes", Kind: config.KindInt, Default: "262144", Min: "1",
			Doc:     "Buffer of the relay per direction and session, in bytes of queued messages.",
			Chapter: chapter24,
		},
		// The windows of a session the host service applies, announced in hello (M2-45, 24
		// sections 6.2 and 6.3); the server's own checks with them follow in M2-46.
		{
			Name: "terminal.idle_timeout", Kind: config.KindDuration, Default: "30m", Min: "1s",
			Doc:     "Without input from the device for this long, the session ends; announced to the host service in hello.",
			Chapter: chapter24,
		},
		{
			Name: "terminal.detach_grace", Kind: config.KindDuration, Default: defaultQuarterHour, Min: "1s",
			Doc:     "This long a detached session stays attachable; announced to the host service in hello.",
			Chapter: chapter24,
		},
		// The server's own limits of a session (M2-46, 24 sections 6.2 and 6.3): the relay and the
		// job terminal.sweep end sessions without the help of the host service, and terminal.open
		// counts the open sessions of the person and of the host.
		{
			Name: "terminal.idle_grace", Kind: config.KindDuration, Default: "2m", Min: "1s",
			Doc:     "Grace after terminal.idle_timeout after which the server itself ends an idle session.",
			Chapter: chapter24,
		},
		{
			Name: "terminal.max_session_duration", Kind: config.KindDuration, Default: "12h", Min: "1s",
			Doc:     "Longest duration of a session from its opening, also across reattaching.",
			Chapter: chapter24,
		},
		{
			Name: "terminal.max_sessions_per_person", Kind: config.KindInt, Default: "5", Min: "1",
			Doc:     "Open sessions of one person; terminal.open is refused with session_limit at this count.",
			Chapter: chapter24,
		},
		{
			Name: "terminal.max_sessions_per_host", Kind: config.KindInt, Default: "10", Min: "1",
			Doc:     "Open sessions on one host; terminal.open is refused with session_limit at this count.",
			Chapter: chapter24,
		},
		{
			Name: "terminal.sweep_interval", Kind: config.KindDuration, Default: thirtySeconds, Min: "1s",
			Doc:     "Interval of the job terminal.sweep.",
			Chapter: chapter24,
		},
		{
			Name: "terminal.host_duplicate_probe_timeout", Kind: config.KindDuration, Default: "5s", Min: minDeadline,
			Doc:     "Time the server waits for the answer of the older control connection of a host (24 section 3.4).",
			Chapter: chapter24,
		},
	}
}

// The keys of the start check fingerprint_keys (M2-41).
const (
	keyFingerprintConfirmChars = "terminal.fingerprint_confirm_chars"
	keyFingerprintMaxAttempts  = "terminal.fingerprint_max_attempts"
)

// fingerprintKeys requires terminal.fingerprint_confirm_chars between 1 and the length of the
// compact fingerprint and terminal.fingerprint_max_attempts of at least 1 (M2-41, Design note 12);
// the error names the key, never the value.
func fingerprintKeys(v *config.Values) error {
	var problems []error
	if n := v.Int(keyFingerprintConfirmChars); n < 1 || n > terminaldomain.FingerprintChars {
		problems = append(problems, errors.New(keyFingerprintConfirmChars+" must be between 1 and the length of the fingerprint"))
	}
	if v.Int(keyFingerprintMaxAttempts) < 1 {
		problems = append(problems, errors.New(keyFingerprintMaxAttempts+" must be at least 1"))
	}
	return errors.Join(problems...)
}

// twentySeconds is the default of terminal.host_heartbeat_interval (24 section 17).
const twentySeconds = "20s"

// sixtySeconds is the default of the step-up windows of the terminal (24 section 17).
const sixtySeconds = "60s"
