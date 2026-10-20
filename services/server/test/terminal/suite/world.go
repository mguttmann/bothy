// SPDX-License-Identifier: Apache-2.0

package suite

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/OWNER/bothy/services/server/test/canary"
	"github.com/OWNER/bothy/services/server/test/harness"
	"github.com/OWNER/bothy/services/server/test/terminal/device"
	"github.com/OWNER/bothy/services/server/test/terminal/stack"
)

// pollEvery is the polling period of every asynchronous effect (Design note 8).
const pollEvery = 250 * time.Millisecond

// hostReadyWait bounds the bring-up of one host after confirmHost (a test constant).
const hostReadyWait = 90 * time.Second

// Env is what the cases of one e2e package share: the stack, the bootstrap owner, the dialer of
// the instance origin, the binaries of the host containers and of the sidecar, the profile and the
// operations of the API.
type Env struct {
	Stack       *stack.Stack
	Server      *harness.Server
	Dialer      *device.Dialer
	Bins        stack.Binaries
	Sidecar     string
	Repo        Repo
	ProfileName string
	Profile     Profile
	Ops         map[string]Operation
	// HostSlots bounds the host containers that start at once (a test constant).
	HostSlots chan struct{}
}

// person is a person of a world instance with the clients of its devices.
type person struct {
	name   string
	client *harness.Client
}

// hostInst is a host of a world instance.
type hostInst struct {
	spec        HostSpec
	id          string
	container   *stack.Host
	pairingCode string
	fingerprint string
	noiseKey    []byte
}

// sessInst is a terminal session of a world instance as the device knows it.
type sessInst struct {
	id, host   string
	device     string
	token      string
	streamPath string
	expires    time.Time
	conns      []*device.Conn
	// connDevice names the device of each connection.
	connDevice []string
}

// instance is the world of one case.
type instance struct {
	t    *testing.T
	env  *Env
	sc   *Scenario
	c    Case
	sfx  string
	ctx  context.Context
	from time.Time

	mu       sync.Mutex
	people   map[string]*person
	devices  map[string]*harness.Client
	groups   map[string]string
	agents   map[string]*harness.Client
	apps     map[string]*harness.Client
	hosts    map[string]*hostInst
	grants   map[string]string
	sessions map[string]*sessInst
	keep     map[string]string
	markers  map[string]string
	fakes    map[string]*fakeHost
	baseline map[string]float64
	// canaries and canaryNames are the terminal canaries of TM-T06 (${canary:<name>}).
	canaries    *canary.Registry
	canaryNames map[string]string
	// statuses are the statuses of the openings by session name (parallel openings, ID-T28).
	statuses map[string]int
}

// newInstance builds the world of sc for case c: people with unique handles (the case's Owner
// made an owner by the bootstrap owner), devices, groups, agents, apps, hosts with their
// containers and grants. Every failure fails the case.
func newInstance(t *testing.T, env *Env, sc *Scenario, c Case) *instance {
	t.Helper()
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	w := &instance{t: t, env: env, sc: sc, c: c, sfx: hex.EncodeToString(b), ctx: t.Context(), from: time.Now().Add(-time.Second),
		people: map[string]*person{}, devices: map[string]*harness.Client{}, groups: map[string]string{}, agents: map[string]*harness.Client{},
		apps: map[string]*harness.Client{}, hosts: map[string]*hostInst{}, grants: map[string]string{}, sessions: map[string]*sessInst{},
		keep: map[string]string{}, markers: map[string]string{}, fakes: map[string]*fakeHost{}, baseline: map[string]float64{},
		statuses: map[string]int{}}
	ww := sc.World
	w.addPerson("owner")
	w.makeOwner("owner")
	for _, p := range ww.People {
		w.addPerson(p)
	}
	for _, o := range ww.Owners {
		w.makeOwner(o)
	}
	for _, d := range ww.Devices {
		w.addDevice(d)
	}
	for _, a := range ww.Agents {
		w.agents[a] = env.Server.NewAgent(t, harness.AgentSpec{Handle: a + "-" + w.sfx, DisplayName: display(a)})
	}
	for _, a := range ww.Apps {
		w.addApp(a)
	}
	var hosts []HostSpec
	for _, h := range ww.Hosts {
		if len(c.Hosts) == 0 || slices.Contains(c.Hosts, h.Name) {
			hosts = append(hosts, h)
		}
	}
	w.addHosts(hosts)
	for _, f := range ww.FakeHosts {
		if len(c.Hosts) == 0 || slices.Contains(c.Hosts, f.Name) {
			w.addFakeHost(f)
		}
	}
	// Groups follow the hosts, so a membership that expires counts from a world that is ready.
	for _, g := range ww.Groups {
		w.addGroup(g)
	}
	for _, g := range ww.Grants {
		if _, ok := w.hosts[g.Host]; !ok {
			if _, fok := w.fakes[g.Host]; !fok {
				continue
			}
		}
		w.addGrant(g)
	}
	return w
}

// display returns the display name of a world name: the name with a capital first letter.
func display(name string) string {
	if name == "" {
		return name
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

// owner returns the client of the case's Owner.
func (w *instance) owner() *harness.Client { return w.people["owner"].client }

// mustCreate sends r as c, with a step-up when the server asks for one, and fails the case unless
// the status is 201; it returns the answer.
func (w *instance) mustCreate(c *harness.Client, r harness.Request) harness.Response {
	const want = http.StatusCreated
	w.t.Helper()
	resp, err := harness.DoWithStepUp(w.ctx, c, r)
	if err != nil {
		w.t.Fatalf("%s/%s: world: %s %s: %v", w.sc.ID, w.c.Name, r.Method, r.Path, err)
	}
	if resp.Status != want {
		w.t.Fatalf("%s/%s: world: %s %s answered %d %s, want %d", w.sc.ID, w.c.Name, r.Method, r.Path, resp.Status, problemCode(resp.Body), want)
	}
	return resp
}

// addPerson creates a person with a local account through the bootstrap owner.
func (w *instance) addPerson(name string) {
	w.t.Helper()
	c, err := w.env.Server.CreateUser(w.ctx, harness.UserSpec{Handle: name + "-" + w.sfx, DisplayName: display(name)})
	if err != nil {
		w.t.Fatalf("%s/%s: world: person %s: %v", w.sc.ID, w.c.Name, name, err)
	}
	w.people[name] = &person{name: name, client: c}
	w.devices[name] = c
}

// makeOwner makes the person an owner (addOwner by the bootstrap owner).
func (w *instance) makeOwner(name string) {
	w.t.Helper()
	w.mustCreate(w.env.Server.Owner, harness.Request{Method: http.MethodPost, Path: "/owners",
		Body: map[string]any{"user_id": w.people[name].client.PrincipalID, "reason": "terminal suite"}})
}

// addDevice enrolls a further device of a person: a recovery code of the case's Owner
// (createEnrollmentCode) redeemed with fresh device keys (redeemEnrollmentCode, 08 3.2).
func (w *instance) addDevice(d DeviceSpec) {
	w.t.Helper()
	p, ok := w.people[d.Person]
	if !ok {
		w.t.Fatalf("%s/%s: world: device %s of unknown person %s", w.sc.ID, w.c.Name, d.Name, d.Person)
	}
	resp := w.mustCreate(w.owner(), harness.Request{Method: http.MethodPost, Path: "/users/" + p.client.PrincipalID + "/enrollment-codes"})
	var code struct {
		Code string `json:"code"`
	}
	resp.JSON(w.t, &code)
	c := w.env.Server.NewDevice()
	r := c.DoAnonymous(w.ctx, harness.Request{Method: http.MethodPost, Path: "/auth/enroll/redeem",
		Body: map[string]any{"enrollment_code": code.Code, "device": c.Registration(display(d.Person) + " second device")}})
	if r.Status != http.StatusCreated {
		w.t.Fatalf("%s/%s: world: redeemEnrollmentCode for %s answered %d %s", w.sc.ID, w.c.Name, d.Name, r.Status, problemCode(r.Body))
	}
	if err := c.Adopt(r.Body); err != nil {
		w.t.Fatal(err)
	}
	c.PrincipalID = p.client.PrincipalID
	w.devices[d.Name] = c
}

// addGroup creates a group with its members (createGroup, addGroupMember).
func (w *instance) addGroup(g GroupSpec) {
	w.t.Helper()
	var grp struct {
		ID string `json:"id"`
	}
	w.mustCreate(w.owner(), harness.Request{Method: http.MethodPost, Path: "/groups", Body: map[string]any{"name": display(g.Name) + " " + w.sfx}}).JSON(w.t, &grp)
	for _, m := range g.Members {
		body := map[string]any{"user_id": w.people[m].client.PrincipalID}
		if g.Expires != "" {
			body["expires_at"] = w.inFuture(g.Expires)
		}
		w.mustCreate(w.owner(), harness.Request{Method: http.MethodPost, Path: "/groups/" + grp.ID + "/members", Body: body})
	}
	w.groups[g.Name] = grp.ID
}

// inFuture returns now plus the duration d as an API timestamp.
func (w *instance) inFuture(d string) string {
	dur, err := ParseDuration(d)
	if err != nil {
		w.t.Fatal(err)
	}
	return time.Now().Add(dur).UTC().Format("2006-01-02T15:04:05.000Z")
}

// addApp installs the app of the world (a minimal manifest with one read scope and no addresses)
// and creates an API token of its principal.
func (w *instance) addApp(name string) {
	w.t.Helper()
	slug := name + "-" + w.sfx
	var app struct {
		ID string `json:"id"`
	}
	w.mustCreate(w.env.Server.Owner, harness.Request{Method: http.MethodPost, Path: "/apps", Body: map[string]any{
		"manifest": map[string]any{"schema_version": 1, "slug": slug, "name": display(name), "description": "Test app of the terminal suite.",
			"publisher": "Example Org", "version": "1.0.0", "min_api_version": "0.1",
			"scopes": map[string]any{"required": []string{"messages:read"}, "optional": []string{}}},
		"granted_scopes": []string{"messages:read"}, "reason": "terminal suite",
	}}).JSON(w.t, &app)
	w.apps[name] = w.env.Server.NewTokenClient(w.t, &harness.Client{PrincipalID: app.ID})
}

// addGrant creates a grant terminal on a host (createGrant by the case's Owner).
func (w *instance) addGrant(g GrantSpec) {
	w.t.Helper()
	body := map[string]any{"target_kind": "host", "target_id": w.hostID(g.Host), "capabilities": []string{"terminal"}, "reason": "terminal suite"}
	if g.Group != "" {
		body["subject_kind"], body["subject_id"] = "group", w.groups[g.Group]
	} else {
		body["subject_kind"], body["subject_id"] = "user", w.people[g.Subject].client.PrincipalID
	}
	if g.Expires != "" {
		body["expires_at"] = w.inFuture(g.Expires)
	}
	var grant struct {
		ID string `json:"id"`
	}
	w.mustCreate(w.owner(), harness.Request{Method: http.MethodPost, Path: "/grants", Body: body}).JSON(w.t, &grant)
	if g.Name != "" {
		w.grants[g.Name] = grant.ID
	}
}

// hostID returns the ID of a host or Fake-Host of the instance.
func (w *instance) hostID(name string) string {
	w.mu.Lock()
	defer w.mu.Unlock()
	if h, ok := w.hosts[name]; ok {
		return h.id
	}
	if f, ok := w.fakes[name]; ok {
		return f.id
	}
	w.t.Fatalf("%s/%s: unknown host %s", w.sc.ID, w.c.Name, name)
	return ""
}

// addHosts brings the hosts up in parallel, bounded by Env.HostSlots.
func (w *instance) addHosts(specs []HostSpec) {
	w.t.Helper()
	var wg sync.WaitGroup
	errs := make([]error, len(specs))
	for i, spec := range specs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w.env.HostSlots <- struct{}{}
			defer func() { <-w.env.HostSlots }()
			errs[i] = w.addHost(spec)
		}()
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			w.t.Fatalf("%s/%s: world: host %s: %v", w.sc.ID, w.c.Name, specs[i].Name, err)
		}
	}
}

// addHost brings one host up after 16 4.4 and M2-42's README: container, binaries, test CA,
// install, configuration, createHost, pair, confirmHost, then polling getHost until it is active,
// connected and reports every assigned session user.
func (w *instance) addHost(spec HostSpec) error {
	h := &hostInst{spec: spec}
	w.mu.Lock()
	w.hosts[spec.Name] = h
	w.mu.Unlock()
	scope := w.c.Name + "-" + w.sfx
	ctr, err := w.env.Stack.NewHost(w.ctx, scope, spec.Name)
	h.container = ctr
	w.t.Cleanup(func() { _ = ctr.Remove(context.WithoutCancel(w.ctx)) })
	if err != nil {
		return err
	}
	if err := ctr.Provision(w.ctx, w.env.Bins); err != nil {
		return err
	}
	if spec.Pair == "container" {
		return nil
	}
	if err := ctr.Install(w.ctx, spec.SessionUsers); err != nil {
		return err
	}
	if spec.Setup != "" {
		if _, err := ctr.Shell(w.ctx, w.expand(spec.Setup)); err != nil {
			return err
		}
	}
	conf, err := w.hostConfig(spec, spec.SessionUsers)
	if err != nil {
		return err
	}
	if err := ctr.WriteConfig(w.ctx, conf); err != nil {
		return err
	}
	if spec.Pair == "installed" {
		return nil
	}
	// Host names are unique per instance, so every world names its hosts with its suffix.
	body := map[string]any{"name": spec.Name + "-" + w.sfx, "reason": "terminal suite"}
	if spec.AssignedTo != "" {
		body["assigned_user_id"] = w.people[spec.AssignedTo].client.PrincipalID
	}
	resp, err := harness.DoWithStepUp(w.ctx, w.owner(), harness.Request{Method: http.MethodPost, Path: "/hosts", Body: body})
	if err != nil {
		return fmt.Errorf("createHost: %w", err)
	}
	if resp.Status != http.StatusCreated {
		return fmt.Errorf("createHost: %d %s", resp.Status, problemCode(resp.Body))
	}
	var created struct {
		HostID      string `json:"host_id"`
		PairingCode string `json:"pairing_code"`
	}
	if err := json.Unmarshal(resp.Body, &created); err != nil {
		return err
	}
	w.mu.Lock()
	h.id, h.pairingCode = created.HostID, created.PairingCode
	w.mu.Unlock()
	if spec.Pair == "unpaired" {
		return nil
	}
	fp, code, err := ctr.Pair(w.ctx, created.PairingCode)
	if err != nil {
		return fmt.Errorf("pair: %w", err)
	}
	if code != 0 {
		return fmt.Errorf("pair exited %d", code)
	}
	w.mu.Lock()
	h.fingerprint = fp
	w.mu.Unlock()
	if spec.Pair == "unconfirmed" {
		return nil
	}
	if err := w.confirm(h, fp); err != nil {
		return err
	}
	return w.waitHostReady(h)
}

// confirm sends confirmHost by the case's Owner with the first terminal.fingerprint_confirm_chars
// characters of the fingerprint and the declarations of the host.
func (w *instance) confirm(h *hostInst, fp string) error {
	chars, err := w.env.Profile.Int("terminal.fingerprint_confirm_chars")
	if err != nil {
		chars = 8 // the default of 24 17 when the profile leaves it unset
	}
	plain := strings.ReplaceAll(fp, "-", "")
	if len(plain) < chars {
		return fmt.Errorf("the fingerprint has %d characters", len(plain))
	}
	body := map[string]any{"fingerprint_prefix": plain[:chars], "llm_credentials": orDefault(h.spec.LLMCredentials, "organization"),
		"declared_server_host": h.spec.DeclaredServerHost, "reason": "terminal suite"}
	if h.spec.CredentialHolder != "" {
		body["credential_holder_id"] = w.people[h.spec.CredentialHolder].client.PrincipalID
	}
	var su []any
	for _, p := range slices.Sorted(maps.Keys(h.spec.Assign)) {
		su = append(su, map[string]any{"user_id": w.people[p].client.PrincipalID, "os_user": h.spec.Assign[p]})
	}
	if len(su) > 0 {
		body["session_users"] = su
	}
	// The host service reports while the host waits for its confirmation, and each report raises
	// the version of the host, so a conflict of If-Match is read again (a few times).
	for attempt := 0; ; attempt++ {
		get, err := harness.DoWithStepUp(w.ctx, w.owner(), harness.Request{Method: http.MethodGet, Path: "/hosts/" + h.id})
		if err != nil {
			return fmt.Errorf("getHost: %w", err)
		}
		if get.Status != http.StatusOK {
			return fmt.Errorf("getHost: %d", get.Status)
		}
		resp, err := harness.DoWithStepUp(w.ctx, w.owner(), harness.Request{Method: http.MethodPost, Path: "/hosts/" + h.id + "/confirm",
			Header: http.Header{"If-Match": []string{get.Header.Get("ETag")}}, Body: body})
		if err != nil {
			return fmt.Errorf("confirmHost: %w", err)
		}
		if resp.Status == http.StatusPreconditionFailed && attempt < confirmRetries {
			continue
		}
		if resp.Status != http.StatusOK {
			return fmt.Errorf("confirmHost: %d %s", resp.Status, problemCode(resp.Body))
		}
		return nil
	}
}

// confirmRetries bounds the reads again after a conflict of If-Match (a test constant).
const confirmRetries = 5

// orDefault returns v or, when empty, d.
func orDefault(v, d string) string {
	if v == "" {
		return d
	}
	return v
}

// waitHostReady polls getHost until the host is active, connected and reports every assigned
// session user.
func (w *instance) waitHostReady(h *hostInst) error {
	deadline := time.Now().Add(hostReadyWait)
	var last string
	for time.Now().Before(deadline) {
		resp := w.owner().Do(w.ctx, harness.Request{Method: http.MethodGet, Path: "/hosts/" + h.id})
		var host struct {
			State     string `json:"state"`
			Connected bool   `json:"connected"`
			Reported  []struct {
				OSUser string `json:"os_user"`
			} `json:"reported_session_users"`
			NoiseKey string `json:"noise_public_key"`
		}
		if resp.Status == http.StatusOK && json.Unmarshal(resp.Body, &host) == nil {
			reported := map[string]bool{}
			for _, r := range host.Reported {
				reported[r.OSUser] = true
			}
			ok := host.State == "active" && host.Connected
			for _, u := range h.spec.Assign {
				ok = ok && reported[u]
			}
			last = fmt.Sprintf("state %s, connected %v, %d reported", host.State, host.Connected, len(host.Reported))
			if ok {
				key, err := b64url(host.NoiseKey)
				if err != nil {
					return err
				}
				w.mu.Lock()
				h.noiseKey = key
				w.mu.Unlock()
				return nil
			}
		}
		select {
		case <-w.ctx.Done():
			return w.ctx.Err()
		case <-time.After(pollEvery):
		}
	}
	return fmt.Errorf("the host did not become ready within %s (%s)", hostReadyWait, last)
}

// hostConfig renders /etc/bothy-host/config.yaml: the session users, the host-service keys of the
// profile, then those of the host.
func (w *instance) hostConfig(spec HostSpec, users []string) ([]byte, error) {
	hs := map[string]any{"session_users": users}
	for k, v := range w.env.Profile.HostService {
		hs[strings.TrimPrefix(k, "host_service.")] = v
	}
	for k, v := range spec.Config {
		hs[strings.TrimPrefix(k, "host_service.")] = w.expandAny(v)
	}
	return yaml.Marshal(map[string]any{"host_service": hs})
}
