// SPDX-License-Identifier: Apache-2.0

package demo

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"
)

// timestampLayout is the form of a timestamp in a request: RFC 3339 in UTC with milliseconds
// (06 "Basic rules").
const timestampLayout = "2006-01-02T15:04:05.000Z"

// agentTokenName is the name of the API token of a demo agent.
const agentTokenName = "Demo agent"

// host creates the demo host (createHost, host.manage with step-up) and a grant terminal on it
// for every person with a session user. With the exchange directory it hands the pairing code to
// the stack, waits for the fingerprint the host printed and confirms the host (pairHost runs on
// the host); without it the host stays pairing (16 16.5).
func (r *runner) host(ctx context.Context) error {
	h := r.ds.Host
	var out struct {
		HostID      string `json:"host_id"`
		PairingCode string `json:"pairing_code"`
	}
	if err := r.owner.ownerPost(ctx, "createHost", "/hosts", map[string]any{"name": h.Name, "reason": h.Reason}, http.StatusCreated, &out); err != nil {
		return err
	}
	r.res.HostID = out.HostID
	for _, u := range h.SessionUsers {
		body := map[string]any{"subject_kind": "user", "subject_id": r.res.People[u.Person], "target_kind": "host", "target_id": out.HostID,
			"capabilities": []string{"terminal"}, "reason": grantReason}
		if err := r.owner.ownerPost(ctx, "createGrant", "/grants", body, http.StatusCreated, nil); err != nil {
			return err
		}
	}
	if r.o.Exchange == "" {
		return nil
	}
	users := make([]string, 0, len(h.SessionUsers))
	for _, u := range h.SessionUsers {
		users = append(users, u.OSUser)
	}
	if err := writePrivate(r.o.Exchange, HostPairingFile, HostPairingDoc{HostID: out.HostID, PairingCode: out.PairingCode, SessionUsers: users}); err != nil {
		return err
	}
	bound := time.Duration(r.ds.Values.HostWait)
	var paired HostPairedDoc
	err := waitFor(ctx, r.o.Sleep, bound, "the paired host", func() (bool, error) {
		doc, ok, err := readPaired(r.o.Exchange)
		paired = doc
		return ok, err
	})
	if err != nil {
		return err
	}
	if err := r.waitReported(ctx, out.HostID, users, bound); err != nil {
		return err
	}
	return r.confirmHost(ctx, out.HostID, paired.Fingerprint)
}

// hostView is the part of getHost the command reads.
type hostView struct {
	State    string `json:"state"`
	Reported []struct {
		OSUser string `json:"os_user"`
	} `json:"reported_session_users"`
}

// waitReported waits until the host reports every session user of the data set: confirmHost
// refuses a session user the host did not report (24 2.2).
func (r *runner) waitReported(ctx context.Context, hostID string, users []string, bound time.Duration) error {
	return waitFor(ctx, r.o.Sleep, bound, "the session users of the host", func() (bool, error) {
		var v hostView
		if err := r.owner.withStepUp(ctx, func() error { return r.owner.get(ctx, "getHost", "/hosts/"+hostID, nil, &v) }); err != nil {
			return false, err
		}
		for _, u := range users {
			if !slices.ContainsFunc(v.Reported, func(x struct {
				OSUser string `json:"os_user"`
			}) bool {
				return x.OSUser == u
			}) {
				return false, nil
			}
		}
		return true, nil
	})
}

// confirmRetries bounds the reads again after a 412 of confirmHost: the host service reports
// while it waits for the confirmation, and each report raises the version of the host.
const confirmRetries = 5

// confirmHost confirms the host with the first FingerprintChars characters of the fingerprint the
// host printed (24 3.2: the owner types what the host shows), declares the LLM login none and
// assigns the session users.
func (r *runner) confirmHost(ctx context.Context, hostID, fingerprint string) error {
	plain := strings.ReplaceAll(fingerprint, "-", "")
	if len(plain) < r.o.FingerprintChars || r.o.FingerprintChars <= 0 {
		return errors.New("demo: the fingerprint of the host is too short")
	}
	assign := make([]map[string]any, 0, len(r.ds.Host.SessionUsers))
	for _, u := range r.ds.Host.SessionUsers {
		assign = append(assign, map[string]any{"user_id": r.res.People[u.Person], "os_user": u.OSUser})
	}
	body := map[string]any{"fingerprint_prefix": plain[:r.o.FingerprintChars], "llm_credentials": "none", "declared_server_host": false,
		"session_users": assign, "reason": r.ds.Host.Reason}
	path := "/hosts/" + hostID
	for attempt := 0; ; attempt++ {
		err := r.owner.withStepUp(ctx, func() error {
			var etag string
			if err := r.owner.do(ctx, call{op: "getHost", method: http.MethodGet, path: path, want: http.StatusOK, etag: &etag}); err != nil {
				return err
			}
			return r.owner.do(ctx, call{op: "confirmHost", method: http.MethodPost, path: path + "/confirm", body: body, idempotent: true,
				want: http.StatusOK, ifMatch: etag})
		})
		var se *StatusError
		if errors.As(err, &se) && se.Status == http.StatusPreconditionFailed && attempt < confirmRetries {
			continue
		}
		return err
	}
}

// agentTokens creates an API token bound to a fresh DPoP key for every agent (createToken with
// dpop_jkt, 08 12), writes them with the agent values to agents.json and waits until every agent
// reports itself online (16 16.5). Without the exchange directory it does nothing.
func (r *runner) agentTokens(ctx context.Context) error {
	if r.o.Exchange == "" {
		return nil
	}
	var inst struct {
		ID string `json:"id"`
	}
	if err := r.owner.get(ctx, "getInstance", "/instance", nil, &inst); err != nil {
		return err
	}
	v := r.ds.Values
	doc := AgentsDoc{InstanceID: inst.ID, Values: AgentValues{
		WorkerSteps: v.WorkerSteps, StepIntervalMS: time.Duration(v.StepInterval).Milliseconds(),
		WatcherIntervalMS: time.Duration(v.WatcherInterval).Milliseconds(), HeartbeatIntervalMS: time.Duration(v.HeartbeatInterval).Milliseconds(),
		TokensIn: v.TokensIn, TokensOut: v.TokensOut,
	}}
	expires := time.Now().Add(time.Duration(v.AgentTokenTTL)).UTC().Format(timestampLayout)
	for _, a := range r.ds.Agents {
		k, err := newKey()
		if err != nil {
			return err
		}
		var tok struct {
			Secret string `json:"secret"`
		}
		body := map[string]any{"name": agentTokenName, "dpop_jkt": k.thumbprint(), "expires_at": expires}
		if err := r.owner.ownerPost(ctx, "createToken", "/principals/"+r.res.Agents[a.Handle]+"/tokens", body, http.StatusCreated, &tok); err != nil {
			return err
		}
		doc.Agents = append(doc.Agents, AgentEntry{
			Handle: a.Handle, Role: a.Role, Provider: a.Provider, Model: a.Model, Currency: r.currency(a),
			AgentID: r.res.Agents[a.Handle], Token: tok.Secret, DPoPPrivateJWK: k.privateJWK(),
		})
	}
	if err := writePrivate(r.o.Exchange, AgentsFile, doc); err != nil {
		return err
	}
	return waitFor(ctx, r.o.Sleep, time.Duration(v.AgentWait), "the demo agents", func() (bool, error) {
		for _, a := range r.ds.Agents {
			var ag struct {
				Presence struct {
					State string `json:"state"`
				} `json:"presence"`
			}
			if err := r.owner.get(ctx, "getAgent", "/agents/"+r.res.Agents[a.Handle], nil, &ag); err != nil {
				return false, err
			}
			if ag.Presence.State != "online" {
				return false, nil
			}
		}
		return true, nil
	})
}

// currency returns the currency of the prices of the model of a (check guarantees one exists).
func (r *runner) currency(a Agent) string {
	for _, p := range r.ds.Prices {
		if p.Provider == a.Provider && p.Model == a.Model {
			return p.Currency
		}
	}
	return ""
}

// warmup posts the warm-up of the seed (Plan), each prompt as a new thread in its channel written
// by the person the seed picked, Pace apart: the operator of the channel answers each with a
// worker run, and the measurement builds the history (16 16.4).
func (r *runner) warmup(ctx context.Context) error {
	for i, p := range r.ds.Plan(r.o.Seed) {
		if i > 0 {
			if err := r.o.Sleep(ctx, r.o.Pace); err != nil {
				return err
			}
		}
		if _, err := r.newThread(ctx, r.channels[channelKey(p.Space, p.Channel)], p.Person, p.Title, p.Text); err != nil {
			return err
		}
	}
	return nil
}

// live posts the live prompts; the runs of their reviewer and watcher operators stay open.
func (r *runner) live(ctx context.Context) error {
	for _, l := range r.ds.Live {
		id, err := r.newThread(ctx, r.channels[channelKey(l.Space, l.Channel)], l.Person, l.Title, l.Text)
		if err != nil {
			return err
		}
		r.res.LiveThreads[l.Title] = id
	}
	return nil
}
