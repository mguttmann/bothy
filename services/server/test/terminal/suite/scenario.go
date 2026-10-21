// SPDX-License-Identifier: Apache-2.0

package suite

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Scenario is one file scenarios/<id>.yaml (Design note 7): the world of the scenario and its
// cases. Every expectation in it is written literally from 24 14 and the chapters it names.
type Scenario struct {
	// ID equals the file name without .yaml (tm-t03, az-m54-61, ...).
	ID string `yaml:"id"`
	// Title names the row of 24 14 or the matrix rows.
	Title string `yaml:"title"`
	// Sources name the places of the specification the expectations come from.
	Sources []string `yaml:"sources"`
	// Profile is the stack profile (default, limits, twonode).
	Profile string `yaml:"profile"`
	// Sequential runs the cases one after the other: they change state the stack shares (the
	// routing of the TLS front, a restart of the server).
	Sequential bool `yaml:"sequential"`
	// World is instantiated once per case.
	World World `yaml:"world"`
	// Cases run in parallel, each in its own world.
	Cases []Case `yaml:"cases"`
}

// World is the world of a scenario: people, owners, devices, groups, agents, apps, hosts and
// grants. Every case gets fresh instances with unique handles.
type World struct {
	// People are the persons of the world by name (alex, sam, robin, ...); their display names
	// are the names with a capital letter. The case's Owner (name owner) always exists.
	People []string `yaml:"people"`
	// Owners are people made owners with addOwner by the bootstrap owner, besides owner.
	Owners []string `yaml:"owners"`
	// Devices are further devices of people (a second enrollment, 08 3.2).
	Devices []DeviceSpec `yaml:"devices"`
	// Groups have members.
	Groups []GroupSpec `yaml:"groups"`
	// Agents are agent principals with an API token (createAgent, createToken).
	Agents []string `yaml:"agents"`
	// Apps are installed apps with an API token (installApp, createToken).
	Apps []string `yaml:"apps"`
	// Hosts are host containers with a real host service.
	Hosts []HostSpec `yaml:"hosts"`
	// Grants are grants of the right terminal on hosts.
	Grants []GrantSpec `yaml:"grants"`
	// FakeHosts are M2-44's Fake-Host connected through the TLS front (TM-T11).
	FakeHosts []FakeHostSpec `yaml:"fake_hosts"`
}

// DeviceSpec is a further device of a person.
type DeviceSpec struct {
	Name   string `yaml:"name"`
	Person string `yaml:"person"`
}

// GroupSpec is a group with members; Expires optionally lets every membership expire after a
// duration from the start of the case.
type GroupSpec struct {
	Name    string   `yaml:"name"`
	Members []string `yaml:"members"`
	Expires string   `yaml:"expires_in"`
}

// HostSpec is one host of the world.
type HostSpec struct {
	// Name is the host name (r1, r2, ...), also the name in createHost.
	Name string `yaml:"name"`
	// AssignedTo is the person the host is assigned to (empty: no holder).
	AssignedTo string `yaml:"assigned_to"`
	// LLMCredentials is organization (default), personal or none.
	LLMCredentials string `yaml:"llm_credentials"`
	// CredentialHolder is the holder of personal credentials.
	CredentialHolder string `yaml:"credential_holder"`
	// DeclaredServerHost is declared_server_host of confirmHost.
	DeclaredServerHost bool `yaml:"declared_server_host"`
	// SessionUsers are installed on the machine (host_service.session_users), the first with
	// install, the others with session-user add.
	SessionUsers []string `yaml:"session_users"`
	// Assign maps people to session users in confirmHost.
	Assign map[string]string `yaml:"assign"`
	// Config are host-service keys of the case on top of the profile.
	Config map[string]any `yaml:"config"`
	// Setup is a root shell script run after install, before the configuration is written.
	Setup string `yaml:"setup"`
	// Pair: paired (default: pair and confirm), unconfirmed (paired, not confirmed), unpaired
	// (install and createHost, not paired), installed (install and configuration, no createHost)
	// or container (only the container with the binaries).
	Pair string `yaml:"pair"`
}

// FakeHostSpec is a Fake-Host of M2-44 paired through pairHost (TM-T11).
type FakeHostSpec struct {
	Name       string            `yaml:"name"`
	AssignedTo string            `yaml:"assigned_to"`
	Assign     map[string]string `yaml:"assign"`
	// NeverIdle keeps the Fake-Host from ever reporting idle.
	NeverIdle bool `yaml:"never_idle"`
}

// GrantSpec is a grant of terminal on a host to a person or a group.
type GrantSpec struct {
	Name    string `yaml:"name"`
	Subject string `yaml:"subject"`
	Group   string `yaml:"group"`
	Host    string `yaml:"host"`
	Expires string `yaml:"expires_in"`
}

// Case is one named list of steps; its world is fresh.
type Case struct {
	Name string `yaml:"name"`
	// Hosts limits the hosts of the world this case starts (empty: all).
	Hosts []string `yaml:"hosts"`
	Steps []Step   `yaml:"steps"`
}

// Step is one step of a case: exactly one kind (Kind).
type Step struct {
	// Call: as a principal, an operation of docs/api/openapi.yaml.
	As      string            `yaml:"as"`
	Call    string            `yaml:"call"`
	Params  map[string]string `yaml:"params"`
	With    any               `yaml:"with"`
	IfMatch string            `yaml:"if_match"`
	StepUp  bool              `yaml:"step_up"`

	// Device: a device of a person (open, attach, connect, ...).
	Device       string     `yaml:"device"`
	Do           string     `yaml:"do"`
	On           string     `yaml:"on"`
	Session      string     `yaml:"session"`
	Profile      string     `yaml:"profile"`
	WorkdirIndex *int       `yaml:"workdir_index"`
	Text         string     `yaml:"text"`
	Token        *TokenSpec `yaml:"token"`
	Ephemeral    string     `yaml:"ephemeral"`
	From         *int       `yaml:"from"`

	// Host: a host container (exec, exec_as, restart_service, reboot, ...).
	Host   string   `yaml:"host"`
	Run    []string `yaml:"run"`
	Script string   `yaml:"script"`
	User   string   `yaml:"user"`
	To     string   `yaml:"to"`
	// Until repeats an exec until its expectation holds (a change on the machine that comes later,
	// like the run of a crontab entry), within the bound of a check.
	Until bool `yaml:"until"`

	// Server: restart, front_mode or sql.
	Server  string `yaml:"server"`
	Service string `yaml:"service"`
	Mode    string `yaml:"mode"`
	SQL     string `yaml:"sql"`
	Bypass  bool   `yaml:"bypass"`

	// Wait: one configuration key or a list whose profile values add up.
	Wait any `yaml:"wait"`

	// Check: session, host, audit, processes, markers, canary, metric, readyz.
	Check string `yaml:"check"`
	// Marker names the marker argument of background processes (processes checks).
	Marker string `yaml:"marker"`

	// Parallel runs its steps at once.
	Parallel []Step `yaml:"parallel"`
	// Sequence runs its steps in order (a lane of a parallel step).
	Sequence []Step `yaml:"sequence"`

	// Expect holds the literal expectations; Keep binds names to values of the answer.
	Expect *Expect           `yaml:"expect"`
	Keep   map[string]string `yaml:"keep"`
}

// TokenSpec selects the session token of a handshake: the own token (default), the token of
// another session (Of), or a forged one (Forge: foreign_kid, invocation_typ).
type TokenSpec struct {
	Of    string `yaml:"of"`
	Forge string `yaml:"forge"`
}

// Expect is the union of the expectations of all step kinds; Validate allows per kind only its
// fields.
type Expect struct {
	// API answers.
	Status *int          `yaml:"status"`
	Code   string        `yaml:"code"`
	Reason string        `yaml:"reason"`
	Errors []ErrorExpect `yaml:"errors"`
	// Fields maps JSON pointers of the body to literal values.
	Fields map[string]any `yaml:"fields"`
	// Absent lists JSON pointers that must not exist in the body.
	Absent []string `yaml:"absent"`
	// Lengths maps JSON pointers of lists in the body to their length.
	Lengths map[string]int `yaml:"lengths"`
	// Statuses is the multiset of the statuses of the openings of a parallel step, in any order.
	Statuses []int `yaml:"statuses"`

	// Stream: the close of the device stream.
	Close *CloseExpect `yaml:"close"`
	// Output must appear in the output of the session (expect_output).
	Output string `yaml:"output"`
	// Replay asks for one replay_begin and replay_end pair on an attach.
	Replay *bool `yaml:"replay"`

	// Commands: exit code and output of a command in a host container.
	Exit           *int     `yaml:"exit"`
	ExitNot        *int     `yaml:"exit_not"`
	StdoutContains []string `yaml:"stdout_contains"`
	StdoutLacks    []string `yaml:"stdout_lacks"`
	StderrContains []string `yaml:"stderr_contains"`

	// session: state, close_reason, close_detail, exit_code of getTerminalSession.
	State       string `yaml:"state"`
	CloseReason string `yaml:"close_reason"`
	CloseDetail string `yaml:"close_detail"`
	ExitCode    *int   `yaml:"exit_code"`

	// host: getHost and the rows of the host.
	Connected            *bool         `yaml:"connected"`
	Doctor               []DoctorEntry `yaml:"doctor"`
	DoctorLacks          []DoctorEntry `yaml:"doctor_lacks"`
	ReportedSessionUsers []string      `yaml:"reported_session_users"`
	// Workdirs maps a reported session user to the display names of its reported working
	// directories, in order.
	Workdirs map[string][]string `yaml:"workdirs"`
	// Profiles are the names of the reported profiles, in any order; ProfilesLack names profiles
	// that must not be reported.
	Profiles       []string          `yaml:"profiles"`
	ProfilesLack   []string          `yaml:"profiles_lack"`
	SessionUsers   map[string]string `yaml:"session_users"`
	DisabledReason string            `yaml:"disabled_reason"`
	NoKeys         *bool             `yaml:"no_keys"`
	Unit           *UnitExpect       `yaml:"unit"`

	// audit: entries of audit_events, of the actor and the resource named here (names of the
	// world: a person, a host, a session, a grant), written since the start of the case.
	Actor      string         `yaml:"actor"`
	Resource   string         `yaml:"resource"`
	Action     string         `yaml:"action"`
	Decision   string         `yaml:"decision"`
	DenyReason string         `yaml:"deny_reason"`
	Details    map[string]any `yaml:"details"`
	Count      *int           `yaml:"count"`
	AtLeast    *int           `yaml:"at_least"`

	// processes: counts of the processes of a session user in a host container.
	Shells    *int     `yaml:"shells"`
	Markers   *int     `yaml:"markers"`
	Processes *int     `yaml:"processes"`
	Users     []string `yaml:"users"`

	// metric: the rise of a counter in the collector export.
	Metric string            `yaml:"metric"`
	Attrs  map[string]string `yaml:"attrs"`
	Rise   *int              `yaml:"rise"`

	// readyz: the readiness of the api containers.
	Ready *bool `yaml:"ready"`

	// relay_nodes: the number of api nodes whose relay paired a session of the case (the pairing
	// records of the relay in their logs, TM-T15).
	NodesAtLeast *int `yaml:"nodes_at_least"`

	// canary: no finding in any sink, and the positive control seen.
	Findings *int `yaml:"findings"`

	// Journal must contain each code in the journal of the bothy-host units.
	Journal []string `yaml:"journal"`
}

// ErrorExpect is one entry of errors of a Problem.
type ErrorExpect struct {
	Code    string `yaml:"code"`
	Pointer string `yaml:"pointer"`
}

// CloseExpect is the close code and reason of a stream.
type CloseExpect struct {
	Code   int    `yaml:"code"`
	Reason string `yaml:"reason"`
}

// DoctorEntry is an entry of Host.doctor (or of hosts.reported_doctor).
type DoctorEntry struct {
	Check       string   `yaml:"check"`
	OK          *bool    `yaml:"ok"`
	Code        string   `yaml:"code"`
	CodeIn      []string `yaml:"code_in"`
	SessionUser *string  `yaml:"session_user"`
}

// UnitExpect is the state of the systemd unit bothy-host.service (systemctl show).
type UnitExpect struct {
	NRestarts      *int  `yaml:"n_restarts"`
	ExecMainStatus *int  `yaml:"exec_main_status"`
	Active         *bool `yaml:"active"`
}

// Step kinds.
const (
	KindCall     = "call"
	KindDevice   = "device"
	KindHost     = "host"
	KindServer   = "server"
	KindWait     = "wait"
	KindCheck    = "check"
	KindParallel = "parallel"
	KindSequence = "sequence"
)

// The do values of device and host steps and the values of server and check steps.
var (
	deviceOps = []string{"enroll", "step_up", "open", "attach", "connect", "handshake", "start", "send", "expect_output", "replay_message1", "cut", "close", "wait_close"}
	hostOps   = []string{"exec", "exec_as", "restart_service", "reboot", "pause", "unpause", "net_down", "net_up", "clone_state", "doctor", "pair"}
	serverOps = []string{"restart", "front_mode", "sql"}
	checkOps  = []string{"session", "host", "audit", "processes", "markers", "canary", "metric", "readyz", "relay_nodes"}
)

// ErrScenario reports a scenario file that breaks the rules of Design note 7.
var ErrScenario = errors.New("scenario")

// Kind returns the kind of s, or an error when s has none or more than one.
func (s Step) Kind() (string, error) {
	var kinds []string
	if s.Call != "" || s.As != "" {
		kinds = append(kinds, KindCall)
	}
	if s.Check != "" {
		kinds = append(kinds, KindCheck)
	} else {
		if s.Device != "" && s.Do != "" {
			kinds = append(kinds, KindDevice)
		}
		if s.Host != "" && s.Do != "" && s.Device == "" {
			kinds = append(kinds, KindHost)
		}
	}
	if s.Server != "" {
		kinds = append(kinds, KindServer)
	}
	if s.Wait != nil {
		kinds = append(kinds, KindWait)
	}
	if len(s.Parallel) > 0 {
		kinds = append(kinds, KindParallel)
	}
	if len(s.Sequence) > 0 {
		kinds = append(kinds, KindSequence)
	}
	if len(kinds) != 1 {
		return "", fmt.Errorf("%w: a step has %d kinds %v, want exactly one", ErrScenario, len(kinds), kinds)
	}
	return kinds[0], nil
}

// WaitKeys returns the configuration keys of a wait step.
func (s Step) WaitKeys() ([]string, error) {
	switch w := s.Wait.(type) {
	case string:
		return []string{w}, nil
	case []any:
		var out []string
		for _, x := range w {
			k, ok := x.(string)
			if !ok {
				return nil, fmt.Errorf("%w: wait takes configuration keys", ErrScenario)
			}
			out = append(out, k)
		}
		return out, nil
	}
	return nil, fmt.Errorf("%w: wait takes a configuration key or a list of keys", ErrScenario)
}

// LoadScenario decodes one scenario file strictly (known fields only).
//
// Errors: a decode error naming the field, or ErrScenario.
func LoadScenario(path string) (*Scenario, error) {
	b, err := os.ReadFile(path) //nolint:gosec // G304: a scenario file of the suite
	if err != nil {
		return nil, err
	}
	var sc Scenario
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&sc); err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	if want := strings.TrimSuffix(filepath.Base(path), ".yaml"); sc.ID != want {
		return nil, fmt.Errorf("%w: %s: id %q differs from the file name", ErrScenario, filepath.Base(path), sc.ID)
	}
	return &sc, nil
}

// Rules are the inputs of Validate that come from the repository: the operations of
// docs/api/openapi.yaml, the keys of docs/operations/configuration.md and the profiles.
type Rules struct {
	Operations map[string]Operation
	ConfigKeys map[string]bool
	Profiles   *Profiles
}

// sqlWrite matches statements that write.
var sqlWrite = regexp.MustCompile(`(?i)^\s*(insert|update|delete|alter|drop|create|truncate|grant|revoke|begin|with\s.*\b(insert|update|delete)\b)`)

// Validate checks sc against the rules of Design note 7 and returns every problem.
func Validate(sc *Scenario, r Rules) []string {
	var probs []string
	add := func(format string, args ...any) { probs = append(probs, sc.ID+": "+fmt.Sprintf(format, args...)) }
	prof, ok := r.Profiles.Profiles[sc.Profile]
	if !ok {
		add("unknown profile %q", sc.Profile)
	}
	if sc.Title == "" || len(sc.Sources) == 0 {
		add("title and sources are required")
	}
	if len(sc.Cases) == 0 {
		add("no case")
	}
	names := map[string]bool{}
	for _, c := range sc.Cases {
		if c.Name == "" || names[c.Name] {
			add("case name %q empty or twice", c.Name)
		}
		names[c.Name] = true
		if len(c.Steps) == 0 {
			add("%s: no step", c.Name)
			continue
		}
		if !endsWithCheck(c.Steps[len(c.Steps)-1]) {
			add("%s: the last step is no check", c.Name)
		}
		for i, s := range c.Steps {
			for _, p := range validateStep(s, r, prof, sc.ID) {
				add("%s step %d: %s", c.Name, i, p)
			}
		}
	}
	return probs
}

// endsWithCheck reports whether s is a check, a sequence that ends with one, or a parallel step
// whose every lane ends with one (Multiply).
func endsWithCheck(s Step) bool {
	switch {
	case s.Check != "":
		return true
	case len(s.Sequence) > 0:
		return endsWithCheck(s.Sequence[len(s.Sequence)-1])
	case len(s.Parallel) > 0:
		for _, p := range s.Parallel {
			if !endsWithCheck(p) {
				return false
			}
		}
		return true
	}
	return false
}

// validateStep returns the problems of one step.
func validateStep(s Step, r Rules, prof Profile, id string) []string {
	var probs []string
	kind, err := s.Kind()
	if err != nil {
		return []string{err.Error()}
	}
	switch kind {
	case KindCall:
		if _, ok := r.Operations[s.Call]; !ok {
			probs = append(probs, fmt.Sprintf("call %q is no operation of docs/api/openapi.yaml", s.Call))
		}
		if s.As == "" {
			probs = append(probs, "a call needs as")
		}
		if s.Expect == nil || s.Expect.Status == nil {
			probs = append(probs, "a call needs expect with status")
		}
	case KindDevice:
		if !slices.Contains(deviceOps, s.Do) {
			probs = append(probs, fmt.Sprintf("unknown device op %q", s.Do))
		}
		if slices.Contains([]string{"open", "attach", "connect", "handshake", "replay_message1", "wait_close"}, s.Do) && s.Expect == nil {
			probs = append(probs, "a stream step needs expect")
		}
	case KindHost:
		if !slices.Contains(hostOps, s.Do) {
			probs = append(probs, fmt.Sprintf("unknown host op %q", s.Do))
		}
	case KindServer:
		if !slices.Contains(serverOps, s.Server) {
			probs = append(probs, fmt.Sprintf("unknown server op %q", s.Server))
		}
		if s.Server == "sql" {
			if sqlWrite.MatchString(s.SQL) && !s.Bypass {
				probs = append(probs, "a writing sql step needs bypass: true")
			}
			if s.Bypass && id != "tm-t28" {
				probs = append(probs, "bypass is allowed only in tm-t28.yaml")
			}
		}
	case KindWait:
		keys, err := s.WaitKeys()
		if err != nil {
			probs = append(probs, err.Error())
		}
		for _, k := range keys {
			if !r.ConfigKeys[k] {
				probs = append(probs, fmt.Sprintf("wait key %q is no row of docs/operations/configuration.md", k))
			}
			if _, ok := prof.Value(k); !ok {
				probs = append(probs, fmt.Sprintf("wait key %q is not set in the profile", k))
			}
		}
	case KindCheck:
		if !slices.Contains(checkOps, s.Check) {
			probs = append(probs, fmt.Sprintf("unknown check %q", s.Check))
		}
		if s.Expect == nil {
			probs = append(probs, "a check needs expect")
		}
	case KindParallel, KindSequence:
		for i, p := range append(slices.Clone(s.Parallel), s.Sequence...) {
			for _, q := range validateStep(p, r, prof, id) {
				probs = append(probs, fmt.Sprintf("%s %d: %s", kind, i, q))
			}
		}
	}
	return probs
}
