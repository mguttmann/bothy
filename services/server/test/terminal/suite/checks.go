// SPDX-License-Identifier: Apache-2.0

package suite

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/test/harness"
	"github.com/OWNER/bothy/services/server/test/terminal/stack"
)

// checkWait bounds the polling of one check (a test constant; the windows of the specification are
// passed with wait steps before).
const checkWait = 90 * time.Second

// poll runs f every pollEvery until it returns no mismatch or checkWait passes, and returns the
// last mismatches.
func (w *instance) poll(f func() []string) []string {
	ctx, cancel := w.deadline(checkWait)
	defer cancel()
	for {
		msgs := f()
		if len(msgs) == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return msgs
		case <-time.After(pollEvery):
		}
	}
}

// check runs a check step.
func (w *instance) check(s Step) []string {
	exp := *s.Expect
	switch s.Check {
	case "session":
		return w.poll(func() []string { return w.checkSession(s, exp) })
	case "host":
		return w.poll(func() []string { return w.checkHost(s, exp) })
	case "audit":
		return w.poll(func() []string { return w.checkAudit(exp) })
	case "processes":
		return w.poll(func() []string { return w.checkProcesses(s, exp) })
	case "markers":
		return w.checkMarkers(s, exp)
	case "canary":
		return w.checkCanary(exp)
	case "metric":
		return w.poll(func() []string { return w.checkMetric(exp) })
	case "readyz":
		return w.checkReadyz(exp)
	}
	return []string{"unknown check " + s.Check}
}

// checkSession reads getTerminalSession as the device of the session (or as the case's Owner).
func (w *instance) checkSession(s Step, exp Expect) []string {
	ses, err := w.session(s.Session)
	if err != nil {
		return []string{err.Error()}
	}
	c, err := w.client(orDefault(s.Device, ses.device))
	if err != nil {
		return []string{err.Error()}
	}
	resp := c.Do(w.ctx, harness.Request{Method: http.MethodGet, Path: "/terminal/sessions/" + ses.id})
	if resp.Status != http.StatusOK {
		return []string{fmt.Sprintf("getTerminalSession answered %d %s", resp.Status, problemCode(resp.Body))}
	}
	var obs ObservedSession
	if err := json.Unmarshal(resp.Body, &obs); err != nil {
		return []string{"getTerminalSession: no JSON"}
	}
	return CompareSession(exp, obs)
}

// checkHost reads getHost as the case's Owner, and when exp asks for them hosts.disabled_reason
// and the state of bothy-host.service.
func (w *instance) checkHost(s Step, exp Expect) []string {
	id := w.hostID(s.Host)
	if id == "" {
		// A machine without a host record of its own (a clone of TM-T23): only its journal.
		h, err := w.container(s.Host)
		if err != nil {
			return []string{err.Error()}
		}
		j, err := h.Journal(w.ctx)
		if err != nil {
			return []string{err.Error()}
		}
		return CompareJournal(exp.Journal, j)
	}
	resp := w.owner().Do(w.ctx, harness.Request{Method: http.MethodGet, Path: "/hosts/" + id})
	if resp.Status != http.StatusOK {
		return []string{fmt.Sprintf("getHost answered %d %s", resp.Status, problemCode(resp.Body))}
	}
	var obs ObservedHost
	if err := json.Unmarshal(resp.Body, &obs); err != nil {
		return []string{"getHost: no JSON"}
	}
	if exp.DisabledReason != "" {
		rows, err := w.env.Stack.SQL(w.ctx, "SELECT coalesce(disabled_reason, '') AS disabled_reason FROM hosts WHERE id = '"+id+"'", false)
		if err != nil || len(rows) != 1 {
			return []string{fmt.Sprintf("hosts.disabled_reason: %d rows, %v", len(rows), err)}
		}
		obs.DisabledReason = fmt.Sprint(rows[0]["disabled_reason"])
	}
	if exp.Unit != nil {
		u, err := w.unitState(s.Host)
		if err != nil {
			return []string{err.Error()}
		}
		obs.Unit = u
	}
	msgs := CompareHost(exp, obs, w.peopleIDs())
	if len(exp.Journal) > 0 {
		h, err := w.container(s.Host)
		if err != nil {
			return append(msgs, err.Error())
		}
		j, err := h.Journal(w.ctx)
		if err != nil {
			return append(msgs, err.Error())
		}
		msgs = append(msgs, CompareJournal(exp.Journal, j)...)
	}
	return msgs
}

// unitState reads NRestarts, ExecMainStatus and ActiveState of bothy-host.service.
func (w *instance) unitState(host string) (*ObservedUnit, error) {
	h, err := w.container(host)
	if err != nil {
		return nil, err
	}
	r, err := h.Exec(w.ctx, nil, "systemctl", "show", "-p", "NRestarts", "-p", "ExecMainStatus", "-p", "ActiveState", "bothy-host.service")
	if err != nil {
		return nil, err
	}
	u := &ObservedUnit{}
	for _, l := range strings.Split(string(r.Stdout), "\n") {
		k, v, _ := strings.Cut(strings.TrimSpace(l), "=")
		switch k {
		case "NRestarts":
			u.NRestarts, _ = strconv.Atoi(v)
		case "ExecMainStatus":
			u.ExecMainStatus, _ = strconv.Atoi(v)
		case "ActiveState":
			u.Active = v == "active" || v == "activating" || v == "reloading"
		}
	}
	return u, nil
}

// peopleIDs maps the person names of the instance to their IDs.
func (w *instance) peopleIDs() map[string]string {
	out := map[string]string{}
	for n, p := range w.people {
		out[n] = p.client.PrincipalID
	}
	return out
}

// container returns the host container of a host name.
func (w *instance) container(host string) (*stack.Host, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	h, ok := w.hosts[host]
	if !ok || h.container == nil {
		return nil, fmt.Errorf("unknown host container %s", host)
	}
	return h.container, nil
}

// resourceID returns the ID of a world name for the audit filter.
func (w *instance) resourceID(name string) (string, error) {
	v, err := w.lookup(name, "id")
	if err != nil {
		return "", err
	}
	return v, nil
}

// checkAudit reads the audit entries of the case (since its start, of the actor and the resource
// of exp) with read-only SQL and compares them.
func (w *instance) checkAudit(exp Expect) []string {
	q := "SELECT action, decision, coalesce(deny_reason, '') AS deny_reason, details FROM audit_events WHERE occurred_at >= '" +
		w.from.UTC().Format(time.RFC3339Nano) + "'"
	if exp.Actor != "" {
		id, err := w.resourceID(exp.Actor)
		if err != nil {
			return []string{err.Error()}
		}
		q += " AND actor_principal_id = '" + id + "'"
	}
	if exp.Resource != "" {
		id, err := w.resourceID(exp.Resource)
		if err != nil {
			return []string{err.Error()}
		}
		q += " AND resource_id = '" + id + "'"
	}
	if exp.Actor == "" && exp.Resource == "" {
		return []string{"an audit check names actor or resource"}
	}
	if exp.Action != "" {
		q += " AND action = '" + strings.ReplaceAll(exp.Action, "'", "") + "'"
	}
	rows, err := w.env.Stack.SQL(w.ctx, q, false)
	if err != nil {
		return []string{err.Error()}
	}
	obs := make([]ObservedAudit, 0, len(rows))
	for _, r := range rows {
		b, _ := json.Marshal(r)
		var a ObservedAudit
		_ = json.Unmarshal(b, &a)
		obs = append(obs, a)
	}
	return CompareAudit(exp, obs)
}

// psLine is one process of ps in a host container.
type psLine struct {
	user      string
	pid, ppid int
	args      string
}

// processes reads every process of a host container.
func (w *instance) processes(host string) ([]psLine, error) {
	h, err := w.container(host)
	if err != nil {
		return nil, err
	}
	r, err := h.Exec(w.ctx, nil, "ps", "-eo", "user:32=,pid=,ppid=,args=")
	if err != nil {
		return nil, err
	}
	var out []psLine
	sc := bufio.NewScanner(bytes.NewReader(r.Stdout))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) < 4 {
			continue
		}
		pid, _ := strconv.Atoi(f[1])
		ppid, _ := strconv.Atoi(f[2])
		out = append(out, psLine{user: f[0], pid: pid, ppid: ppid, args: strings.Join(f[3:], " ")})
	}
	return out, nil
}

// countProcesses counts, for the session user u, its shells and its processes other than its PTY
// helper, the processes of any user with the marker argument, and the users of every process below
// the shells of u.
func countProcesses(ps []psLine, u, marker string) ObservedProcesses {
	var obs ObservedProcesses
	children := map[int][]psLine{}
	for _, p := range ps {
		children[p.ppid] = append(children[p.ppid], p)
	}
	for _, p := range ps {
		if marker != "" && strings.Contains(" "+p.args+" ", " "+marker+" ") && !strings.HasPrefix(p.args, "ps ") {
			obs.Markers++
		}
		if p.user != u || strings.Contains(p.args, "pty-helper") {
			continue
		}
		obs.Processes++
		if strings.HasPrefix(p.args, "-") {
			obs.Shells++
			stack := []psLine{p}
			for len(stack) > 0 {
				q := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				obs.Users = append(obs.Users, q.user)
				stack = append(stack, children[q.pid]...)
			}
		}
	}
	return obs
}

// checkProcesses compares the processes of a session user in a host container.
func (w *instance) checkProcesses(s Step, exp Expect) []string {
	ps, err := w.processes(s.Host)
	if err != nil {
		return []string{err.Error()}
	}
	marker := ""
	if s.Marker != "" {
		marker = w.expand(s.Marker)
	}
	return CompareProcesses(exp, countProcesses(ps, s.User, marker))
}

// checkMarkers counts the marker files of the programs that must never run (Design note 5).
func (w *instance) checkMarkers(s Step, exp Expect) []string {
	h, err := w.container(s.Host)
	if err != nil {
		return []string{err.Error()}
	}
	r, err := h.Exec(w.ctx, nil, "find", stack.MarkerDir, "-mindepth", "1", "-type", "f")
	if err != nil {
		return []string{err.Error()}
	}
	n := 0
	for _, l := range strings.Split(strings.TrimSpace(string(r.Stdout)), "\n") {
		if l != "" {
			n++
		}
	}
	return CompareMarkers(exp, n)
}

// metricValue sums the data points of a counter with the attributes attrs in the metrics export
// of the collector (the file overlay of M1-73).
func (w *instance) metricValue(name string, attrs map[string]string) (float64, error) {
	dst := filepath.Join(w.t.TempDir(), "metrics.jsonl")
	if err := w.env.Stack.CopyFrom(w.ctx, "otel-collector", "/var/lib/otelcol-smoke/metrics.jsonl", dst); err != nil {
		return 0, nil //nolint:nilerr // before the first export the file does not exist: the value is 0
	}
	b, err := os.ReadFile(dst) //nolint:gosec // G304: a file of the test's temp dir
	if err != nil {
		return 0, err
	}
	return SumCounter(b, name, attrs), nil
}

// SumCounter returns the newest cumulative value of the counter name with attrs in an OTLP JSON
// metrics export (one ResourceMetrics document per line), summed over the resources (the api
// nodes): per resource and attribute set the largest value counts, because a cumulative sum only
// grows.
func SumCounter(export []byte, name string, attrs map[string]string) float64 {
	type kv struct {
		Key   string `json:"key"`
		Value struct {
			S string `json:"stringValue"`
		} `json:"value"`
	}
	type point struct {
		Attributes []kv    `json:"attributes"`
		AsInt      string  `json:"asInt"`
		AsDouble   float64 `json:"asDouble"`
	}
	type doc struct {
		ResourceMetrics []struct {
			Resource struct {
				Attributes []kv `json:"attributes"`
			} `json:"resource"`
			ScopeMetrics []struct {
				Metrics []struct {
					Name string `json:"name"`
					Sum  struct {
						DataPoints []point `json:"dataPoints"`
					} `json:"sum"`
				} `json:"metrics"`
			} `json:"scopeMetrics"`
		} `json:"resourceMetrics"`
	}
	best := map[string]float64{}
	sc := bufio.NewScanner(bytes.NewReader(export))
	sc.Buffer(make([]byte, 0, 1<<20), 64<<20)
	for sc.Scan() {
		var d doc
		if json.Unmarshal(sc.Bytes(), &d) != nil {
			continue
		}
		for _, rm := range d.ResourceMetrics {
			res := ""
			for _, a := range rm.Resource.Attributes {
				res += a.Key + "=" + a.Value.S + ";"
			}
			for _, sm := range rm.ScopeMetrics {
				for _, m := range sm.Metrics {
					if m.Name != name {
						continue
					}
					for _, p := range m.Sum.DataPoints {
						got := map[string]string{}
						for _, a := range p.Attributes {
							got[a.Key] = a.Value.S
						}
						ok := true
						for k, v := range attrs {
							ok = ok && got[k] == v
						}
						if !ok {
							continue
						}
						v := p.AsDouble
						if p.AsInt != "" {
							n, _ := strconv.ParseFloat(p.AsInt, 64)
							v = n
						}
						key := res + "|" + fmt.Sprint(slices.Sorted(maps.Keys(got)))
						if v > best[key] {
							best[key] = v
						}
					}
				}
			}
		}
	}
	total := 0.0
	for _, v := range best {
		total += v
	}
	return total
}

// metricKey names a counter with its attributes.
func metricKey(name string, attrs map[string]string) string {
	return name + fmt.Sprint(attrs)
}

// recordBaselines reads, at the start of the case, every counter its metric checks compare.
func (w *instance) recordBaselines() {
	for _, s := range w.c.Steps {
		if s.Check == "metric" && s.Expect != nil {
			v, err := w.metricValue(s.Expect.Metric, s.Expect.Attrs)
			if err != nil {
				w.t.Fatal(err)
			}
			w.baseline[metricKey(s.Expect.Metric, s.Expect.Attrs)] = v
		}
	}
}

// checkMetric compares the rise of a counter since the start of the case.
func (w *instance) checkMetric(exp Expect) []string {
	v, err := w.metricValue(exp.Metric, exp.Attrs)
	if err != nil {
		return []string{err.Error()}
	}
	return CompareMetric(exp, v-w.baseline[metricKey(exp.Metric, exp.Attrs)])
}

// checkReadyz runs the healthcheck of the image in every api container.
func (w *instance) checkReadyz(exp Expect) []string {
	ready := map[string]bool{}
	svcs := []string{"api"}
	if w.env.Stack.Profile == stack.ProfileTwoNode {
		svcs = append(svcs, "api-2")
	}
	for _, svc := range svcs {
		_, _, code, err := w.env.Stack.ComposeRaw(w.ctx, nil, "exec", "-T", svc, "/usr/local/bin/bothy", "healthcheck", "--url", "http://127.0.0.1:8081/readyz")
		ready[svc] = err == nil && code == 0
	}
	return CompareReadyz(exp, ready)
}
