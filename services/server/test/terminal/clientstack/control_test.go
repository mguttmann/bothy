// SPDX-License-Identifier: Apache-2.0

package clientstack

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testToken = "control-token-of-the-test"

func newControl(t *testing.T) (*Control, *[]string) {
	t.Helper()
	var paired []string
	c := &Control{
		Token: testToken,
		Pair: func(_ context.Context, code string) (PairResult, error) {
			paired = append(paired, code)
			if code == "bothy_hpc_brokenbrokenbroken" {
				return PairResult{}, errors.New("docker exec failed")
			}
			return PairResult{Fingerprint: "7KQ2-9XTA-1111", ExitCode: 0}, nil
		},
		CheckCanary: func(context.Context) ([]string, error) {
			return []string{"container_logs: api line 3 content"}, nil
		},
		EnrollmentCode: func(context.Context) (string, error) {
			return "enrollment-code-of-alex", nil
		},
	}
	return c, &paired
}

func call(t *testing.T, c *Control, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequestWithContext(t.Context(), method, path, strings.NewReader(body))
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	c.ServeHTTP(w, r)
	return w
}

func TestControlPairsWithTheCodeOfTheTest(t *testing.T) {
	c, paired := newControl(t)
	w := call(t, c, http.MethodPost, "/pair", testToken, `{"code":"bothy_hpc_abcdefghijklmnop0123"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d %s", w.Code, w.Body)
	}
	var got PairResult
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Fingerprint != "7KQ2-9XTA-1111" || got.ExitCode != 0 {
		t.Fatalf("result %+v", got)
	}
	if len(*paired) != 1 || (*paired)[0] != "bothy_hpc_abcdefghijklmnop0123" {
		t.Fatalf("paired %v", *paired)
	}
}

func TestControlRefusesWithoutTheTokenAndBadInput(t *testing.T) {
	c, paired := newControl(t)
	for _, tc := range []struct {
		name, method, path, token, body string
		want                            int
	}{
		{"no token", http.MethodPost, "/pair", "", `{"code":"bothy_hpc_abcdefghijklmnop0123"}`, http.StatusUnauthorized},
		{"wrong token", http.MethodPost, "/pair", "other", `{"code":"bothy_hpc_abcdefghijklmnop0123"}`, http.StatusUnauthorized},
		{"not a pairing code", http.MethodPost, "/pair", testToken, `{"code":"--server=https://example.com"}`, http.StatusBadRequest},
		{"no JSON", http.MethodPost, "/pair", testToken, `code`, http.StatusBadRequest},
		{"GET", http.MethodGet, "/pair", testToken, ``, http.StatusMethodNotAllowed},
		{"unknown path", http.MethodPost, "/other", testToken, `{}`, http.StatusNotFound},
	} {
		if w := call(t, c, tc.method, tc.path, tc.token, tc.body); w.Code != tc.want {
			t.Errorf("%s: status %d, want %d", tc.name, w.Code, tc.want)
		}
	}
	if len(*paired) != 0 {
		t.Fatalf("a refused request paired: %v", *paired)
	}
}

func TestControlReportsAFailedPairing(t *testing.T) {
	c, _ := newControl(t)
	if w := call(t, c, http.MethodPost, "/pair", testToken, `{"code":"bothy_hpc_brokenbrokenbroken"}`); w.Code != http.StatusBadGateway {
		t.Fatalf("status %d, want 502", w.Code)
	}
}

func TestControlRunsTheCanaryCheck(t *testing.T) {
	c, _ := newControl(t)
	w := call(t, c, http.MethodPost, "/canary-check", testToken, `{}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	var got struct {
		Findings []string `json:"findings"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Findings) != 1 {
		t.Fatalf("findings %v", got.Findings)
	}
}

func TestPublishedPortsReadsComposePs(t *testing.T) {
	// One JSON object per line, as `docker compose ps --format json` prints it.
	ps := `{"Name":"p-front-1","Service":"front","Publishers":[{"URL":"127.0.0.1","TargetPort":8443,"PublishedPort":49200,"Protocol":"tcp"}]}
{"Name":"p-api-1","Service":"api","Publishers":[{"URL":"","TargetPort":8080,"PublishedPort":0,"Protocol":"tcp"}]}
{"Name":"p-e2e-r1","Service":"termhost","Publishers":null}
`
	got, err := PublishedPorts([]byte(ps))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != (Published{Container: "p-front-1", Address: "127.0.0.1", Port: 49200}) {
		t.Fatalf("published %+v", got)
	}
	if err := OnlyFront(got, 49200); err != nil {
		t.Fatal(err)
	}
	for _, other := range [][]Published{
		nil,
		{{Container: "p-front-1", Address: "0.0.0.0", Port: 49200}},
		{got[0], {Container: "p-e2e-r1", Address: "127.0.0.1", Port: 22}},
		{{Container: "p-front-1", Address: "127.0.0.1", Port: 49201}},
	} {
		if OnlyFront(other, 49200) == nil {
			t.Fatalf("%+v passed", other)
		}
	}
	if _, err := PublishedPorts([]byte("not json\n")); err == nil {
		t.Fatal("a broken line passed")
	}
}

func TestE2EWorldLeavesTheHostUnpaired(t *testing.T) {
	w := world(ModeE2E)
	if len(w.Hosts) != 1 || w.Hosts[0].Pair != "installed" || w.Hosts[0].AssignedTo != "" {
		t.Fatalf("e2e host %+v", w.Hosts)
	}
	if len(w.Owners) != 1 || w.Owners[0] != "alex" {
		t.Fatalf("owners %v", w.Owners)
	}
	p := world(ModePerf)
	if p.Hosts[0].Pair != "" || p.Hosts[0].AssignedTo != "alex" || p.Hosts[0].Assign["alex"] != sessionUser {
		t.Fatalf("perf host %+v", p.Hosts)
	}
	s := world(ModeStack)
	if len(s.Grants) != 1 || len(s.Owners) != 0 {
		t.Fatalf("stack world %+v", s)
	}
}

func TestGeneratorProfiles(t *testing.T) {
	profiles := generatorProfiles()
	names := map[string]bool{}
	for _, p := range profiles {
		if m, ok := p.(map[string]any); ok {
			names[m["name"].(string)] = true
			if m["kind"] != "command" || m["program"] != generatorPath {
				t.Fatalf("profile %v", m)
			}
		} else if p != "shell" {
			t.Fatalf("profile %v", p)
		}
	}
	if !names[RateProfile] || !names[BurstProfile] {
		t.Fatalf("names %v", names)
	}
}

func TestControlIssuesAFreshEnrollmentCode(t *testing.T) {
	c, _ := newControl(t)
	w := call(t, c, http.MethodPost, "/enrollment-code", testToken, `{}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	var got struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Code != "enrollment-code-of-alex" {
		t.Fatalf("code %q", got.Code)
	}
	if w := call(t, c, http.MethodPost, "/enrollment-code", "", `{}`); w.Code != http.StatusUnauthorized {
		t.Fatalf("without the token: %d", w.Code)
	}
}
