// SPDX-License-Identifier: Apache-2.0

package conformance

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/test/harness"
	"github.com/OWNER/bothy/services/server/test/terminal/suite"
)

// TestBaselineDoctorPasses proves requirement 26: every end-to-end scenario starts from a host
// that passed doctor after bothy-host install; a freshly installed host exits 0 in bothy-host
// doctor, and Host.doctor has no failed entry.
func TestBaselineDoctorPasses(t *testing.T) {
	t.Parallel()
	env := suite.Get(t)
	l := suite.NewLive(t, env, "baseline", suite.World{
		People: []string{"alex", "sam", "robin"},
		Hosts: []suite.HostSpec{
			{Name: "r1", AssignedTo: "alex", SessionUsers: []string{"term-alex"}, Assign: map[string]string{"alex": "term-alex"}},
			{Name: "r2", AssignedTo: "robin", LLMCredentials: "personal", CredentialHolder: "robin",
				SessionUsers: []string{"term-robin", "term-sam", "term-alex"}, Assign: map[string]string{"robin": "term-robin"}},
		},
	})
	for _, name := range l.Hosts() {
		res, err := l.Host(name).Doctor(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if res.Code != 0 {
			t.Errorf("%s: bothy-host doctor exited %d after install", name, res.Code)
		}
		resp := l.Owner().Do(t.Context(), harness.Request{Method: http.MethodGet, Path: "/hosts/" + l.HostID(name)})
		var host struct {
			Doctor []suite.ObservedDoctor `json:"doctor"`
		}
		if resp.Status != http.StatusOK || json.Unmarshal(resp.Body, &host) != nil {
			t.Fatalf("%s: getHost answered %d", name, resp.Status)
		}
		if len(host.Doctor) == 0 {
			t.Errorf("%s: Host.doctor is empty", name)
		}
		for _, e := range host.Doctor {
			if !e.OK {
				t.Errorf("%s: Host.doctor has a failed entry %s %s", name, e.Check, e.Code)
			}
		}
	}
}

// TestSessionRunsAsSessionUser proves requirements 21 and 26: every process of a shell session,
// its background processes included, runs as the session user of its person, never as root or
// as the service user.
func TestSessionRunsAsSessionUser(t *testing.T) {
	t.Parallel()
	env := suite.Get(t)
	l := suite.NewLive(t, env, "session-user", suite.World{
		People: []string{"alex", "sam"},
		Hosts: []suite.HostSpec{{Name: "r1", AssignedTo: "alex", SessionUsers: []string{"term-alex", "term-sam"},
			Assign: map[string]string{"alex": "term-alex", "sam": "term-sam"}}},
		Owners: []string{"sam"},
		Grants: []suite.GrantSpec{{Subject: "alex", Host: "r1"}},
	})
	background := "setsid sleep ${marker} & nohup sleep ${marker} >/dev/null 2>&1 & "
	l.Do(suite.ShellSteps("alex", "r1", "s1", background)...)
	l.Do(suite.ShellSteps("sam", "r1", "s2", background)...)
	for _, u := range []string{"term-alex", "term-sam"} {
		shells := 1
		l.Do(suite.Step{Check: "processes", Host: "r1", User: u, Expect: &suite.Expect{Shells: &shells, Users: []string{u}}})
	}
}

// TestHostServiceListensOnNoNetworkPort proves requirement 23: in a host container with a running
// session no process holds a TCP or UDP socket on a non-loopback address (ss), no host container
// publishes a port (docker inspect), and from a host container only the TLS front answers, while
// api and postgres do not resolve (the network termhosts reaches nothing else).
func TestHostServiceListensOnNoNetworkPort(t *testing.T) {
	t.Parallel()
	env := suite.Get(t)
	l := suite.NewLive(t, env, "no-listener", suite.World{
		People: []string{"alex"},
		Hosts:  []suite.HostSpec{{Name: "r1", AssignedTo: "alex", SessionUsers: []string{"term-alex"}, Assign: map[string]string{"alex": "term-alex"}}},
		Grants: []suite.GrantSpec{{Subject: "alex", Host: "r1"}},
	})
	l.Do(suite.ShellSteps("alex", "r1", "s1", "")...)
	h := l.Host("r1")
	for _, args := range [][]string{{"ss", "-H", "-ltnup"}, {"ss", "-H", "-lunp"}} {
		res, err := h.Exec(t.Context(), nil, args...)
		if err != nil || res.Code != 0 {
			t.Fatalf("%v exited %d: %v", args, res.Code, err)
		}
		for _, line := range strings.Split(strings.TrimSpace(string(res.Stdout)), "\n") {
			f := strings.Fields(line)
			// With both -t and -u ss prints the column Netid first.
			col := 3
			if len(f) > 0 && (f[0] == "tcp" || f[0] == "udp") {
				col = 4
			}
			if len(f) <= col {
				continue
			}
			local := f[col]
			if !strings.HasPrefix(local, "127.") && !strings.HasPrefix(local, "[::1]") && !strings.HasPrefix(local, "::1") {
				t.Errorf("%v: a socket listens on %s (%s)", args, local, processOf(line))
			}
		}
	}
	for _, format := range []string{"{{json .NetworkSettings.Ports}}", "{{json .HostConfig.PortBindings}}"} {
		v, err := h.Inspect(t.Context(), format)
		if err != nil {
			t.Fatal(err)
		}
		if v != "{}" && v != "null" {
			t.Errorf("the host container publishes ports: %s %s", format, v)
		}
	}
	answers, err := h.Exec(t.Context(), nil, "bash", "-c", "exec 3<>/dev/tcp/chat.example.com/8443")
	if err != nil || answers.Code != 0 {
		t.Errorf("the TLS front chat.example.com:8443 does not answer from the host container (exit %d)", answers.Code)
	}
	for _, name := range []string{"api", "postgres", "worker", "otel-collector"} {
		res, err := h.Exec(t.Context(), nil, "getent", "hosts", name)
		if err != nil {
			t.Fatal(err)
		}
		if res.Code == 0 {
			t.Errorf("%s resolves from a host container", name)
		}
	}
}

// processOf returns the process part of an ss line (users:(("name",pid=...))), without data.
func processOf(line string) string {
	if i := strings.Index(line, "users:"); i >= 0 {
		return line[i:]
	}
	return "no process"
}
