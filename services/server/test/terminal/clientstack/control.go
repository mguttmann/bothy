// SPDX-License-Identifier: Apache-2.0

package clientstack

import (
	"bufio"
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/OWNER/bothy/services/server/test/terminal/stack"
	"github.com/OWNER/bothy/services/server/test/terminal/suite"
)

// Mode is what the helper prepares (BOTHY_CLIENTSTACK_MODE).
type Mode string

// The modes of the helper.
const (
	// ModeStack is the stack test of M4-36 (make terminal-stack): a paired host assigned to Alex
	// with a grant terminal, enrollment codes for two devices of Alex and one of the Owner.
	ModeStack Mode = ""
	// ModeE2E is the terminal E2E of the desktop app (brief M4-38 Design note 1, make
	// e2e-desktop): Alex is an owner and the host container is installed but not paired; the app
	// creates the host in the administration and the control endpoint runs bothy-host pair.
	ModeE2E Mode = "e2e"
	// ModePerf is the terminal performance level (brief M4-38 Design note 2, make perf-desktop):
	// a paired host assigned to Alex, an owner, with the generator profiles.
	ModePerf Mode = "perf"
	// ModeMobile is the terminal suite of the phones (brief M5-13 Design note 1, make
	// e2e-mobile): the host paired through the API, assigned to Alex, an owner, with his session
	// user, named "Build host"; the terminal windows of the profile and the control endpoint for
	// enrollment codes.
	ModeMobile Mode = "mobile"
)

// profileOf returns the profile of profiles.yaml the stack of mode runs: mobile for the phone
// suite (a quick reattach window longer than the bounds of its return from the background),
// default for every other mode.
func profileOf(mode Mode) string {
	if mode == ModeMobile {
		return "mobile"
	}
	return "default"
}

// mobileHostName is the name of the host the phone suite sees (01 5.3 M9).
const mobileHostName = "Build host"

// The host of every mode: one container, one session user (24 2.2).
const (
	hostName    = "r1"
	sessionUser = "term-alex"
)

// The generator of the performance level (termgen/main.go), copied into the host container and
// named as two command profiles of the host service (24 11.1).
const (
	generatorPath = stack.HostBinDir + "/termgen"
	// RateProfile writes RateLines lines, one per RateInterval.
	RateProfile = "termgen-rate"
	// BurstProfile writes BurstChunks chunks of BurstChunkBytes bytes, one per acknowledgement.
	BurstProfile    = "termgen-burst"
	RateInterval    = 100 * time.Millisecond
	RateLines       = 1100
	rateWidth       = 64
	BurstChunkBytes = 256 << 10
	BurstChunks     = 16
	burstWidth      = 80
)

// world returns the world of mode: Alex with the session user term-alex on host r1.
func world(mode Mode) suite.World {
	switch mode {
	case ModeE2E:
		// The host is installed with its configuration, never created on the server: the test
		// creates and pairs it through the administration of M4-37. Owners hold terminal
		// implicitly (24 8.1); the session user is assigned in the wizard (SR12).
		return suite.World{
			People: []string{"alex"},
			Owners: []string{"alex"},
			Hosts:  []suite.HostSpec{{Name: hostName, SessionUsers: []string{sessionUser}, Pair: "installed"}},
		}
	case ModePerf, ModeMobile:
		// Pair empty: createHost, bothy-host pair and confirmHost through the API (24 3.2).
		return suite.World{
			People: []string{"alex"},
			Owners: []string{"alex"},
			Hosts: []suite.HostSpec{{
				Name: hostName, AssignedTo: "alex", SessionUsers: []string{sessionUser},
				Assign: map[string]string{"alex": sessionUser},
			}},
		}
	case ModeStack:
		return suite.World{
			People: []string{"alex"},
			Hosts: []suite.HostSpec{{
				Name: hostName, AssignedTo: "alex", SessionUsers: []string{sessionUser},
				Assign: map[string]string{"alex": sessionUser},
			}},
			Grants: []suite.GrantSpec{{Name: "g", Subject: "alex", Host: hostName}},
		}
	default:
		return suite.World{}
	}
}

// Windows are the terminal windows of the stack's profile that the phone suite waits by (brief
// M5-13 Design note 2): the suite never names a number of its own.
type Windows struct {
	// QuickReattachMs is terminal.quick_reattach_window in milliseconds.
	QuickReattachMs int64 `json:"quick_reattach_window_ms"`
	// DetachGraceMs is terminal.detach_grace in milliseconds.
	DetachGraceMs int64 `json:"detach_grace_ms"`
}

// TerminalWindows returns the windows of p; an error names a key the profile lacks or cannot
// parse.
func TerminalWindows(p suite.Profile) (Windows, error) {
	quick, err := p.Duration("terminal.quick_reattach_window")
	if err != nil {
		return Windows{}, err
	}
	grace, err := p.Duration("terminal.detach_grace")
	if err != nil {
		return Windows{}, err
	}
	return Windows{QuickReattachMs: quick.Milliseconds(), DetachGraceMs: grace.Milliseconds()}, nil
}

// generatorProfiles returns host_service.profiles with shell and the two generator profiles.
func generatorProfiles() []any {
	return []any{
		"shell",
		map[string]any{
			"name": RateProfile, "display_name": "Generator rate", "kind": "command", "program": generatorPath,
			"args": []string{"rate", "--interval", RateInterval.String(), "--lines", strconv.Itoa(RateLines), "--width", strconv.Itoa(rateWidth)},
		},
		map[string]any{
			"name": BurstProfile, "display_name": "Generator burst", "kind": "command", "program": generatorPath,
			"args": []string{"burst", "--chunk-bytes", strconv.Itoa(BurstChunkBytes), "--chunks", strconv.Itoa(BurstChunks), "--width", strconv.Itoa(burstWidth)},
		},
	}
}

// PairResult is the answer of the pairing step: the fingerprint bothy-host pair printed and its
// exit code.
type PairResult struct {
	Fingerprint string `json:"fingerprint"`
	ExitCode    int    `json:"exit_code"`
}

// Control is the loopback endpoint of the modes e2e and perf for the steps the app's test cannot
// do itself (brief M4-38 Scope): POST /pair runs bothy-host pair in the host container of the
// project with the code the test read from the pairing wizard and answers the printed
// fingerprint; POST /canary-check scans the stack's logs, database dump, collector export and the
// host journal for the canary the test typed (R-CL-17, TM-T06); POST /enrollment-code issues a
// fresh enrollment code of Alex through the case's Owner, so every run of a suite on the stack
// signs in with the newest code (08 3.2). Every request needs the bearer token of the connection
// data. It listens on 127.0.0.1 only and publishes no container port; a step the mode has not
// (nil) answers 404.
type Control struct {
	// Token is the bearer token of the connection data.
	Token string
	// Pair runs the pairing step with a code of the form bothy_hpc_... (mode e2e).
	Pair func(ctx context.Context, code string) (PairResult, error)
	// CheckCanary returns the findings of the canary scan, empty when clean (mode e2e).
	CheckCanary func(ctx context.Context) ([]string, error)
	// EnrollmentCode issues a fresh enrollment code of Alex.
	EnrollmentCode func(ctx context.Context) (string, error)
}

// pairingCode is the form of a pairing code (bothy_hpc_ plus the secret, ADR 0061); anything else
// never reaches docker exec.
var pairingCode = regexp.MustCompile(`^bothy_hpc_[A-Za-z0-9_-]{16,200}$`)

// ServeHTTP implements http.Handler.
func (c *Control) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	want := "Bearer " + c.Token
	if c.Token == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte(want)) != 1 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	steps := map[string]bool{"/pair": c.Pair != nil, "/canary-check": c.CheckCanary != nil, "/enrollment-code": c.EnrollmentCode != nil}
	if !steps[r.URL.Path] {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	switch r.URL.Path {
	case "/pair":
		var body struct {
			Code string `json:"code"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil || !pairingCode.MatchString(body.Code) {
			http.Error(w, "a pairing code bothy_hpc_... is required", http.StatusBadRequest)
			return
		}
		res, err := c.Pair(r.Context(), body.Code)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		writeJSON(w, res)
	case "/enrollment-code":
		code, err := c.EnrollmentCode(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		writeJSON(w, map[string]string{"code": code})
	default:
		findings, err := c.CheckCanary(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		writeJSON(w, map[string]any{"findings": append([]string{}, findings...)})
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// Published is one port a container of the project publishes on the Docker host.
type Published struct {
	Container, Address string
	Port               int
}

// PublishedPorts parses `docker compose ps --all --format json` (one object per line, or one
// array) and returns every published port.
func PublishedPorts(ps []byte) ([]Published, error) {
	type publisher struct {
		URL           string
		PublishedPort int
	}
	type row struct {
		Name       string
		Publishers []publisher
	}
	var rows []row
	trimmed := bytes.TrimSpace(ps)
	if bytes.HasPrefix(trimmed, []byte("[")) {
		if err := json.Unmarshal(trimmed, &rows); err != nil {
			return nil, fmt.Errorf("compose ps: %w", err)
		}
	} else {
		sc := bufio.NewScanner(bytes.NewReader(trimmed))
		sc.Buffer(make([]byte, 0, 64<<10), 4<<20)
		for sc.Scan() {
			if len(bytes.TrimSpace(sc.Bytes())) == 0 {
				continue
			}
			var r row
			if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
				return nil, fmt.Errorf("compose ps: %w", err)
			}
			rows = append(rows, r)
		}
	}
	var out []Published
	for _, r := range rows {
		for _, p := range r.Publishers {
			if p.PublishedPort != 0 {
				out = append(out, Published{Container: r.Name, Address: p.URL, Port: p.PublishedPort})
			}
		}
	}
	return out, nil
}

// OnlyFront returns an error unless pub is exactly the TLS front on 127.0.0.1:front: the stack
// of the client tests publishes nothing else (brief M4-38 Scope).
func OnlyFront(pub []Published, front int) error {
	if len(pub) != 1 || pub[0].Port != front || pub[0].Address != "127.0.0.1" {
		return fmt.Errorf("published ports %+v, want only the TLS front on 127.0.0.1:%d", pub, front)
	}
	return nil
}
