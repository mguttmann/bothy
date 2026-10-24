// SPDX-License-Identifier: Apache-2.0

package importtool

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/OWNER/bothy/services/server/api/restclient"
	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
)

// sessionRefMaxBytes is the maximum of agent.session_ref_max_bytes (docs/operations/
// configuration.md): the tool cannot read the server's configured value, so it refuses only what
// no server accepts, and the server refuses a longer reference than its own value with 400 at
// /items/<i>/data/earlier_session/external_session_ref (docs/DECISIONS.md D-M8-06-1).
// TestSessionRefMaxBytesIsTheKeyMaximum holds the value equal to the key table.
const sessionRefMaxBytes = 2048

// The members of a line of the sessions file (22 9 step 1, 10.1).
const (
	sessMemberThread   = "discord_thread"
	sessMemberAgent    = "agent"
	sessMemberRef      = "external_session_ref"
	sessMemberBaseline = "cost_baseline_micros"
	sessMemberCurrency = "currency"
)

// sessionLine is one line of the sessions file: the Discord thread, the agent handle, the opaque
// reference (never printed or logged) and the cost baseline with its currency ("" when absent).
type sessionLine struct {
	line     int
	thread   string
	agent    string
	ref      string
	baseline int64
	currency string
	// used reports that the thread of the line is part of the run.
	used bool
}

// sessionsFile is the parsed --sessions file by Discord thread ID; nil means no file.
type sessionsFile struct {
	byThread map[string]*sessionLine
	order    []*sessionLine
}

// readSessions reads and parses the file of --sessions (brief design note 1). Errors: a
// usageError (exit 2) naming the line, never the reference.
func readSessions(path string) (*sessionsFile, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- the path of --sessions, chosen by the owner
	if err != nil {
		return nil, finding("the sessions file cannot be read")
	}
	return parseSessions(data)
}

// parseSessions parses JSON lines, one object per line; blank lines are skipped. Every line has
// exactly the members discord_thread (digits), agent (a handle), external_session_ref (non-empty
// display text of at most sessionRefMaxBytes bytes, the text rules of the agent sessions) and
// optionally cost_baseline_micros (an integer from 0) and currency (ISO 4217, required for a
// baseline above 0); a discord_thread appears once. Errors: a usageError naming the line number
// and the member, never a value.
func parseSessions(data []byte) (*sessionsFile, error) {
	f := &sessionsFile{byThread: map[string]*sessionLine{}}
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), len(data)+1)
	n := 0
	for sc.Scan() {
		n++
		text := bytes.TrimSpace(sc.Bytes())
		if len(text) == 0 {
			continue
		}
		l, err := parseSessionLine(text, n)
		if err != nil {
			return nil, err
		}
		if prev := f.byThread[l.thread]; prev != nil {
			return nil, finding("sessions file line %d: discord_thread %s is already named in line %d", n, l.thread, prev.line)
		}
		f.byThread[l.thread] = l
		f.order = append(f.order, l)
	}
	if err := sc.Err(); err != nil {
		return nil, finding("the sessions file cannot be read")
	}
	return f, nil
}

// parseSessionLine checks one line (brief design note 1).
func parseSessionLine(text []byte, n int) (*sessionLine, error) {
	bad := func(member, why string) error {
		return finding("sessions file line %d: %s %s", n, member, why)
	}
	var m map[string]json.RawMessage
	if text[0] != '{' || json.Unmarshal(text, &m) != nil || m == nil {
		return nil, finding("sessions file line %d: not a JSON object", n)
	}
	allowed := []string{sessMemberThread, sessMemberAgent, sessMemberRef, sessMemberBaseline, sessMemberCurrency}
	for _, k := range slices.Sorted(maps.Keys(m)) {
		if !slices.Contains(allowed, k) {
			return nil, finding("sessions file line %d: unknown member %q", n, printable(k))
		}
	}
	l := &sessionLine{line: n}
	var err error
	if l.thread, err = lineString(m, sessMemberThread); err != nil || !snowflake(l.thread) {
		return nil, bad(sessMemberThread, "must be a Discord ID (digits)")
	}
	if l.agent, err = lineString(m, sessMemberAgent); err != nil || strings.TrimSpace(l.agent) == "" {
		return nil, bad(sessMemberAgent, "must be the handle of an agent")
	}
	l.ref, err = lineString(m, sessMemberRef)
	switch {
	case err != nil || strings.TrimSpace(l.ref) == "":
		return nil, bad(sessMemberRef, "must be a non-empty string")
	case textcheck.DisplayText(l.ref) != nil:
		return nil, bad(sessMemberRef, "contains a forbidden character")
	case len(l.ref) > sessionRefMaxBytes:
		return nil, bad(sessMemberRef, "is longer than the largest agent.session_ref_max_bytes")
	}
	if raw, ok := m[sessMemberBaseline]; ok {
		v, perr := strconv.ParseInt(string(bytes.TrimSpace(raw)), 10, 64)
		if perr != nil || v < 0 || v > maxBaselineMicros {
			return nil, bad(sessMemberBaseline, "must be an integer from 0 to 2^53 - 1")
		}
		l.baseline = v
	}
	if _, ok := m[sessMemberCurrency]; ok {
		if l.currency, err = lineString(m, sessMemberCurrency); err != nil || !isoCurrency(l.currency) {
			return nil, bad(sessMemberCurrency, "must be an ISO 4217 code of three capital letters")
		}
	}
	if l.baseline > 0 && l.currency == "" {
		return nil, bad(sessMemberCurrency, "is required for a cost_baseline_micros above 0")
	}
	return l, nil
}

// maxBaselineMicros is the largest cost baseline, 2^53 - 1 (the schema ImportEarlierSession).
const maxBaselineMicros = 1<<53 - 1

// errNotString is the error of a member that is missing or no string.
var errNotString = fmt.Errorf("not a string")

// lineString reads a required string member.
func lineString(m map[string]json.RawMessage, key string) (string, error) {
	raw, ok := m[key]
	if !ok {
		return "", errNotString
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", errNotString
	}
	return s, nil
}

// isoCurrency reports three ASCII capital letters.
func isoCurrency(s string) bool {
	if len(s) != 3 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < 'A' || s[i] > 'Z' {
			return false
		}
	}
	return true
}

// checkAgents holds that every agent handle of the file is an agent of the server (brief design
// note 1): an unknown handle ends the run with exit 2 before any write, naming the line.
func (f *sessionsFile) checkAgents(agents map[string]person) error {
	if f == nil {
		return nil
	}
	for _, l := range f.order {
		if _, ok := agents[l.agent]; !ok {
			return finding("sessions file line %d: agent %s is no agent of the server", l.line, printable(l.agent))
		}
	}
	return nil
}

// earlierSession returns the member earlier_session of the thread item of t in channel c (22 9
// step 1, ImportEarlierSession), or nil: no line for the thread, or a line whose agent is not the
// planned_operator of the channel in the mapping, which is dropped and counted as discarded (22
// 11). It marks the line used and, in a dry run, counts it as "would take over"; an --apply run
// counts the outcome the server answers (tally).
func (wk *walker) earlierSession(c *channelPlan, t *threadPlan) map[string]any {
	l := wk.sessions.line(t.th.ID)
	if l == nil {
		return nil
	}
	l.used = true
	planned := ""
	if c.entry != nil {
		planned = c.entry.PlannedOperator
	}
	agent, known := wk.agents[l.agent]
	if planned == "" || l.agent != planned || !known {
		c.rep.SessionsDiscarded++
		return nil
	}
	if wk.w == nil {
		c.rep.SessionsTaken++
	}
	es := map[string]any{"agent_id": agent.id.String(), "external_session_ref": l.ref}
	if l.baseline > 0 {
		es["cost_baseline_micros"] = l.baseline
	}
	if l.currency != "" {
		es["currency"] = l.currency
	}
	return es
}

// line returns the line of a Discord thread, nil without a file or a line.
func (f *sessionsFile) line(thread string) *sessionLine {
	if f == nil {
		return nil
	}
	return f.byThread[thread]
}

// unused counts the lines whose thread was not part of the run (another channel, --channel,
// --only-thread or no thread of the server); they are ignored.
func (f *sessionsFile) unused() int {
	if f == nil {
		return 0
	}
	n := 0
	for _, l := range f.order {
		if !l.used {
			n++
		}
	}
	return n
}

// tallySession counts the outcome the server answered for the earlier session of a thread item
// (ImportItem.session; M8-06 design note 3): taken over, already present, or late (the thread has
// had a current session since the import, 22 9 step 6).
func tallySession(c *channelReport, it restclient.ImportItem) {
	if it.Session == nil {
		return
	}
	switch *it.Session {
	case restclient.TakenOver:
		c.SessionsTaken++
	case restclient.AlreadyPresent:
		c.SessionsPresent++
	case restclient.DroppedLate:
		c.SessionsLate++
	}
}

// noteSessionsIgnored is the report note of the lines of the sessions file whose thread was not
// part of the run; it carries only the count.
func noteSessionsIgnored(n int) string {
	return fmt.Sprintf("sessions file: %d lines name threads outside this run and were ignored", n)
}
