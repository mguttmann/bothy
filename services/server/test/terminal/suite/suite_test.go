// SPDX-License-Identifier: Apache-2.0

package suite

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/test/canary"
)

// TestCatalogue_Load proves the strict decoding of the catalogue: the repository's catalogue
// loads, and a table of broken documents is rejected (unknown field, duplicate ID, unknown kind or
// os, an ID of the PLAN row with deferred, deferred_parts without ticket).
func TestCatalogue_Load(t *testing.T) {
	repo := mustRepo(t)
	plan := []string{"TM-T03", "TM-T07"}
	c, err := LoadCatalogue(repo.CataloguePath(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Entries) < 29 {
		t.Errorf("the catalogue has %d entries, want at least the 29 IDs of 24 14", len(c.Entries))
	}
	entry := "- id: TM-T03\n  title: t\n  expected: e\n  runs:\n  - {kind: e2e, ref: x}\n"
	broken := map[string]string{
		"unknown field":         "format: 1\nentries:\n- id: TM-T03\n  colour: red\n",
		"duplicate id":          "format: 1\nentries:\n" + entry + entry,
		"unknown kind":          "format: 1\nentries:\n- id: TM-T04\n  runs:\n  - {kind: smoke, ref: x}\n",
		"unknown os":            "format: 1\nentries:\n- id: TM-T13\n  runs:\n  - {kind: platform, ref: TestPlatform_TMT13, os: plan9}\n",
		"deferred plan id":      "format: 1\nentries:\n- id: TM-T07\n  deferred: {ticket: M6-18, reason: r}\n",
		"part without ticket":   "format: 1\nentries:\n- id: TM-T12\n  deferred_parts:\n  - {part: console}\n",
		"wrong format":          "format: 2\nentries: []\n",
		"os on a run of a unit": "format: 1\nentries:\n- id: TM-T01\n  runs:\n  - {kind: unit, ref: x, os: linux}\n",
	}
	for name, doc := range broken {
		if _, err := ParseCatalogue([]byte(doc), plan); err == nil {
			t.Errorf("%s: a broken catalogue loads", name)
		}
	}
}

// rules returns the Rules of the repository.
func rules(t *testing.T, repo Repo) Rules {
	t.Helper()
	ops, err := Operations(repo.Root)
	if err != nil {
		t.Fatal(err)
	}
	keys, err := ConfigKeys(repo.Root)
	if err != nil {
		t.Fatal(err)
	}
	p, err := LoadProfiles(repo.ProfilesPath())
	if err != nil {
		t.Fatal(err)
	}
	return Rules{Operations: ops, ConfigKeys: keys, Profiles: p}
}

// scenarioFiles returns the paths of every file under scenarios/.
func scenarioFiles(t *testing.T, repo Repo) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(repo.SuiteDir(), "scenarios", "*.yaml"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no scenario file: %v", err)
	}
	return files
}

// TestScenarioFiles_Load proves Design note 7 for every file under scenarios/: it loads strictly,
// its id equals the file name, every step has exactly one kind, every call names an operation of
// docs/api/openapi.yaml, every wait key is a row of the configuration reference and set in the
// profile of the file, every call and stream step has an expect, every case ends with a check, and
// a writing sql step needs bypass, which only tm-t28.yaml may use; a table of broken documents is
// rejected.
func TestScenarioFiles_Load(t *testing.T) {
	repo := mustRepo(t)
	r := rules(t, repo)
	for _, f := range scenarioFiles(t, repo) {
		sc, err := LoadScenario(f)
		if err != nil {
			t.Errorf("%s: %v", filepath.Base(f), err)
			continue
		}
		if probs := Validate(sc, r); len(probs) > 0 {
			t.Errorf("%s:\n%s", filepath.Base(f), strings.Join(probs, "\n"))
		}
	}
	head := "id: tm-t99\ntitle: t\nsources: [s]\nprofile: default\nworld: {}\ncases:\n- name: c\n  steps:\n"
	check := "  - {check: markers, host: r1, expect: {markers: 0}}\n"
	broken := map[string]string{
		"two kinds":             head + "  - {as: alex, call: getHost, check: host, expect: {status: 200}}\n" + check,
		"no kind":               head + "  - {expect: {status: 200}}\n" + check,
		"unknown operation":     head + "  - {as: alex, call: makeCoffee, expect: {status: 200}}\n" + check,
		"call without expect":   head + "  - {as: alex, call: getHost}\n" + check,
		"stream without expect": head + "  - {device: alex, do: connect, session: s1}\n" + check,
		"unknown wait key":      head + "  - {wait: terminal.no_such_window}\n" + check,
		"wait key not set":      head + "  - {wait: auth.session_max_age}\n" + check,
		"no final check":        head + "  - {wait: terminal.open_timeout}\n",
		"write without bypass":  head + "  - {server: sql, sql: \"UPDATE hosts SET name = 'x'\"}\n" + check,
		"bypass elsewhere":      head + "  - {server: sql, bypass: true, sql: \"UPDATE hosts SET name = 'x'\"}\n" + check,
		"unknown device op":     head + "  - {device: alex, do: dance, expect: {}}\n" + check,
	}
	for name, doc := range broken {
		path := filepath.Join(t.TempDir(), "tm-t99.yaml")
		if err := os.WriteFile(path, []byte(doc), 0o600); err != nil {
			t.Fatal(err)
		}
		sc, err := LoadScenario(path)
		if err != nil {
			continue
		}
		if probs := Validate(sc, r); len(probs) == 0 {
			t.Errorf("%s: a broken scenario passes", name)
		}
	}
	path := filepath.Join(t.TempDir(), "other.yaml")
	if err := os.WriteFile(path, []byte(head+check), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadScenario(path); err == nil {
		t.Error("a file whose id differs from its name loads")
	}
	unknown := filepath.Join(t.TempDir(), "tm-t99.yaml")
	if err := os.WriteFile(unknown, []byte(head+"  - {check: markers, host: r1, colour: red, expect: {markers: 0}}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadScenario(unknown); err == nil {
		t.Error("an unknown field loads")
	}
}

// Patterns of TestScenarioFiles_NoSecretsAndWhiteLabel.
var (
	compactJWS  = regexp.MustCompile(`eyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]+\.`)
	tokenPrefix = regexp.MustCompile(`\b(bothy_pat_|bothy_hpc_|bothy_rt_|bothy_at_)`)
	pemBlock    = regexp.MustCompile(`-----BEGIN`)
	domainRE    = regexp.MustCompile(`\b[a-z0-9-]+\.(com|net|org|io|de|dev)\b`)
	hostNameRE  = regexp.MustCompile(`\bname: (r[0-9][a-z]?)\b`)
	osUserRE    = regexp.MustCompile(`\bterm-[a-z-]+\b`)
)

// TestScenarioFiles_NoSecretsAndWhiteLabel proves fixed requirements 8 and 20 for every scenario
// file: no compact JWS, no token prefix, no PEM block; only example.com addresses; only the people
// of the world and the neutral host and session user names.
func TestScenarioFiles_NoSecretsAndWhiteLabel(t *testing.T) {
	repo := mustRepo(t)
	people := []string{"owner", "alex", "sam", "robin"}
	for _, f := range scenarioFiles(t, repo) {
		b, err := os.ReadFile(f) //nolint:gosec // G304: a scenario file of the suite
		if err != nil {
			t.Fatal(err)
		}
		text := string(b)
		name := filepath.Base(f)
		for what, re := range map[string]*regexp.Regexp{"a compact JWS": compactJWS, "a token prefix": tokenPrefix, "a PEM block": pemBlock} {
			if re.MatchString(text) {
				t.Errorf("%s holds %s", name, what)
			}
		}
		for _, d := range domainRE.FindAllString(text, -1) {
			if d != "example.com" && !strings.HasSuffix(d, ".example.com") {
				t.Errorf("%s names the address %s", name, d)
			}
		}
		for _, m := range hostNameRE.FindAllStringSubmatch(text, -1) {
			if !regexp.MustCompile(`^r[0-9][a-z]?$`).MatchString(m[1]) {
				t.Errorf("%s names the host %s", name, m[1])
			}
		}
		for _, u := range osUserRE.FindAllString(text, -1) {
			who := strings.TrimPrefix(u, "term-")
			if !slices.Contains(append(people, "ghost", "gone", "robin-gone", "new"), who) {
				t.Errorf("%s names the session user %s", name, u)
			}
		}
		sc, err := LoadScenario(f)
		if err != nil {
			continue
		}
		for _, p := range append(slices.Clone(sc.World.People), sc.World.Owners...) {
			if !slices.Contains(people, p) {
				t.Errorf("%s has the person %s outside Alex, Sam, Robin and the Owner", name, p)
			}
		}
	}
}

// TestCanaryWiringFindsPlantedValue proves the wiring of TM-T06: a canary written into one log line
// on purpose is exactly one finding of the container logs, one in a relay message is one of the
// relay view, the session token in the database dump is one, and the findings never carry a value.
func TestCanaryWiringFindsPlantedValue(t *testing.T) {
	reg := canary.NewRegistry()
	part := reg.New(canary.ClassContent, "", "terminal content part1")
	token := "header.payload-of-a-session-token.signature"
	reg.Register(token, canary.ClassSecret, "", "session token")
	clean := map[string][]byte{SinkContainerLog + ":compose logs": []byte("api | {\"msg\":\"ok\"}\n"), SinkHostJournal + ":r1": []byte("{\"msg\":\"closed\",\"code\":4100}\n")}
	if f := ScanCanaries(reg, CanarySinks(clean, [][]byte{[]byte("ciphertext")})); len(f) != 0 {
		t.Fatalf("clean sources give findings: %v", f)
	}
	planted := map[string][]byte{
		SinkContainerLog + ":compose logs": []byte("api | {\"msg\":\"ok\"}\napi | {\"msg\":\"input " + part + "\"}\n"),
		SinkDatabaseDump + ":pg_dump":      []byte("COPY x FROM stdin;\n" + token + "\n"),
	}
	findings := ScanCanaries(reg, CanarySinks(planted, [][]byte{[]byte("frame " + part)}))
	if len(findings) != 3 {
		t.Fatalf("%d findings, want 3: %v", len(findings), findings)
	}
	all := strings.Join(findings, "\n")
	for _, sink := range []string{SinkContainerLog, SinkDatabaseDump, SinkRelayView} {
		if !strings.Contains(all, sink) {
			t.Errorf("no finding of %s: %s", sink, all)
		}
	}
	if strings.Contains(all, part) || strings.Contains(all, token) {
		t.Error("a finding carries the value")
	}
}

// TestExpectationsBite proves Design note 15: for every kind of expectation and every check kind a
// wrong expected value fails the step, the failure names scenario, case and step index, and it
// carries neither the body nor a token of the observation.
func TestExpectationsBite(t *testing.T) {
	// A made-up compact JWS of low entropy: the test only looks for it in the failures.
	secret := "eyJ" + strings.Repeat("A", 12) + "." + strings.Repeat("B", 12) + ".sig"
	body := []byte(`{"code":"terminal_denied","reason":"not_host_assignee","session_token":"` + secret +
		`","errors":[{"code":"session_user_not_on_host","pointer":"/session_users/0/os_user"}],"session":{"state":"opening"},"items":[1]}`)
	obs := observedCall{Status: 403, Body: body}
	id := func(v any) any { return v }
	n := func(v int) *int { return &v }
	f := false
	tr := true
	cases := map[string][]string{
		"status":       CompareCall(Expect{Status: n(201)}, obs, id),
		"code":         CompareCall(Expect{Code: "step_up_required"}, obs, id),
		"reason":       CompareCall(Expect{Reason: "i6_holder_only"}, obs, id),
		"errors":       CompareCall(Expect{Errors: []ErrorExpect{{Code: "unknown_user"}}}, obs, id),
		"fields":       CompareCall(Expect{Fields: map[string]any{"/session/state": "active"}}, obs, id),
		"absent":       CompareCall(Expect{Absent: []string{"/session/state"}}, obs, id),
		"lengths":      CompareCall(Expect{Lengths: map[string]int{"/items": 2}}, obs, id),
		"close code":   CompareClose(CloseExpect{Code: 4104, Reason: "token_invalid"}, 4101, "token_expired"),
		"close reason": CompareClose(CloseExpect{Code: 4104, Reason: "token_invalid"}, 4104, "session_user_unknown"),
		"replay":       CompareReplay(true, 0, 0),
		"statuses":     CompareStatuses(Expect{Statuses: []int{201, 401}}, []int{201, 201}),
		"session": CompareSession(Expect{State: "closed", CloseReason: "access_revoked", CloseDetail: "token_invalid", ExitCode: n(0)},
			ObservedSession{State: "active"}),
		"host": CompareHost(Expect{State: "disabled", Connected: &tr, NoKeys: &tr, DisabledReason: "duplicate_connection",
			Doctor: []DoctorEntry{{Check: "hd8_separation", OK: &f}}, ReportedSessionUsers: []string{"term-sam"},
			SessionUsers: map[string]string{"alex": "term-alex"}, Unit: &UnitExpect{ExecMainStatus: n(78)}},
			ObservedHost{State: "active", KeyFingerprint: "X", Doctor: []ObservedDoctor{{Check: "hd8_separation", OK: true}}, Unit: &ObservedUnit{}},
			map[string]string{"alex": "a"}),
		"doctor lacks": CompareHost(Expect{DoctorLacks: []DoctorEntry{{Check: "hd6_llm_login", OK: &tr}}},
			ObservedHost{Doctor: []ObservedDoctor{{Check: "hd6_llm_login", OK: true}}}, nil),
		"audit":          CompareAudit(Expect{Action: "terminal.open", Decision: "deny", Details: map[string]any{"code": "x"}}, []ObservedAudit{{Action: "terminal.open", Decision: "deny"}}),
		"audit at least": CompareAudit(Expect{Action: "host.connect", AtLeast: n(2)}, []ObservedAudit{{Action: "host.connect"}}),
		"processes": CompareProcesses(Expect{Shells: n(1), Markers: n(0), Processes: n(0), Users: []string{"term-alex"}},
			ObservedProcesses{Shells: 0, Markers: 2, Processes: 3, Users: []string{"root"}}),
		"markers": CompareMarkers(Expect{Markers: n(0)}, 1),
		"canary":  CompareCanary(Expect{Findings: n(0)}, []string{"log at line 1: content \"x\" (cnry1234...)"}, false),
		"metric":  CompareMetric(Expect{Metric: "bothy.terminal.relay.unpaired", Rise: n(1)}, 0),
		"readyz":  CompareReadyz(Expect{Ready: &tr}, map[string]bool{"api": false}),
		"exec": CompareExec(Expect{Exit: n(0), StdoutContains: []string{"Permission denied"}, StderrContains: []string{"refused"}},
			1, []byte("output "+secret), []byte("")),
		"journal": CompareJournal([]string{`"code":4110`}, []byte(`{"code":4111,"token":"`+secret+`"}`)),
	}
	for name, msgs := range cases {
		if len(msgs) == 0 {
			t.Errorf("%s: a wrong expected value passes", name)
			continue
		}
		msg := Failure("tm-t99", "case_x", 7, msgs)
		if !strings.HasPrefix(msg, "tm-t99/case_x step 7: ") {
			t.Errorf("%s: the failure does not name scenario, case and step: %s", name, msg)
		}
		if strings.Contains(msg, secret) || strings.Contains(msg, "session_token") {
			t.Errorf("%s: the failure carries the body or the token: %s", name, msg)
		}
	}
	if msgs := CompareCall(Expect{Status: n(403), Code: "terminal_denied", Reason: "not_host_assignee",
		Errors: []ErrorExpect{{Code: "session_user_not_on_host", Pointer: "/session_users/0/os_user"}}, Fields: map[string]any{"/session/state": "opening"},
		Lengths: map[string]int{"/items": 1}}, obs, id); len(msgs) > 0 {
		t.Errorf("the right expectation fails: %v", msgs)
	}
}
