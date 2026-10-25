// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	appspg "github.com/OWNER/bothy/services/server/internal/apps/adapters/postgres"
	auditpg "github.com/OWNER/bothy/services/server/internal/audit/adapters/postgres"
	auditapp "github.com/OWNER/bothy/services/server/internal/audit/app"
	billingpg "github.com/OWNER/bothy/services/server/internal/billing/adapters/postgres"
	identitypg "github.com/OWNER/bothy/services/server/internal/identity/adapters/postgres"
	instancepg "github.com/OWNER/bothy/services/server/internal/instance/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/kernel/retention"
	messagingapp "github.com/OWNER/bothy/services/server/internal/messaging/app"
	notifypg "github.com/OWNER/bothy/services/server/internal/notify/adapters/postgres"
	privacypg "github.com/OWNER/bothy/services/server/internal/privacy/adapters/postgres"
	statuspg "github.com/OWNER/bothy/services/server/internal/status/adapters/postgres"
	terminalpg "github.com/OWNER/bothy/services/server/internal/terminal/adapters/postgres"
)

// timeClasses are the classes of 03 whose rows end by time (20 4.1): the six classes of the jobs
// retention.<class> and the classes whose jobs belong to other tickets or modules. config,
// account and personal_sync have no time-based job.
var timeClasses = []string{"security", "content", "transient", "audit", "cost", "privacy_job", "ops", "import", "stats", "event_log"}

// handledElsewhere are the tables of timeClasses that no participant of M7-05 removes, each with
// who does: "job:<kind>" (a job of the catalogue of 02 12.3, checked to exist there),
// "ticket:<id>" (the later ticket that owns the table) or "module:<text>" (the module removes the
// row outside a job).
var handledElsewhere = map[string]string{
	"event_log":            "job:sync.partition_maintenance",
	"idempotency_keys":     "job:sync.idempotency_cleanup",
	"auth_challenges":      "job:identity.auth_cleanup",
	"auth_login_flows":     "job:identity.auth_cleanup",
	"device_pairings":      "job:identity.auth_cleanup",
	"stats_minutes":        "job:stats.retention",
	"file_upload_sessions": "job:files.cleanup_pending",
	"file_upload_parts":    "job:files.cleanup_pending",
	"gateway_presence":     "module:gateway deletes the rows of ended connections and stale nodes (R-DM-51)",
	"import_sources":       "ticket:M8",
	"import_items":         "ticket:M8",
	"dev_environments":     "ticket:V2",
}

// withParent are the tables whose rows go with a parent row by the deletion path of 20 section 2
// (03 section 6.5 for a message, 4.3 for its files): the parent must be covered itself.
var withParent = map[string]string{
	"message_revisions":   "messages",
	"mentions":            "messages",
	"reactions":           "messages",
	"pins":                "messages",
	"message_attachments": "messages",
	"files":               "messages",
	"file_thumbnails":     "files",
	"file_text_extracts":  "files",
	"message_components":  "messages",
	"interactions":        "message_components",
	"thread_tags":         "threads",
	"agent_sessions":      "threads",
	"runs":                "threads",
	"run_steps":           "runs",
	"run_followups":       "runs",
	"dm_participants":     "channels",
}

// withoutEndTime are the tables of timeClasses whose rows carry no end time a job could read: the
// structure of 03 principle 7 that is never removed by time, and rows that go with the account or
// the agent (20 section 2).
var withoutEndTime = map[string]string{
	"spaces":                 "archiving; never removed by time (03 principle 7)",
	"channels":               "archiving; never removed by time (03 principle 7)",
	"threads":                "never removed by time (03 principle 7); its messages go by retention.content",
	"forum_tags":             "changed by the owner of the channel",
	"message_templates":      "personal ones with the account (M7-06)",
	"local_credentials":      "with the account (M7-06)",
	"agent_consumer_cursors": "with the agent",
	"status_components":      "the live configuration of the status page; the owner, status.probe and agents.AgentDisabled remove rows (M7-09)",
}

// retentionParticipants returns the participants internal/wire registers, built without storage:
// Tables and Class need none.
func retentionParticipants(t *testing.T) []retention.Participant {
	t.Helper()
	trim, err := auditapp.NewTrim(fakeUnits{}, auditpg.NewRepository(nil), &auditapp.Recorder{})
	if err != nil {
		t.Fatal(err)
	}
	return []retention.Participant{
		identitypg.NewRetention(nil), instancepg.NewRepository(nil).Retention(), notifypg.NewRegistrations(nil),
		terminalpg.NewRetention(nil), appspg.NewRetention(nil), billingpg.NewRetention(nil),
		new(messagingapp.Service).Retention(), trim, auditapp.NewIPKeys(auditpg.NewRepository(nil), nil),
		statuspg.NewRetention(nil), privacypg.NewRetention(nil),
	}
}

// fakeUnits is a MaintUnits that is never run: the coverage test reads only Tables and Class.
type fakeUnits struct{}

func (fakeUnits) Do(context.Context, func(context.Context) error) error { return nil }

// retentionLine matches a table heading of 03 section 4 and the class of its Retention line.
var (
	tableHeading = regexp.MustCompile("^#### `([a-z_]+)`$")
	retentionOf  = regexp.MustCompile("Retention: `([a-z_]+)`")
	jobRow       = regexp.MustCompile("^\\| `([a-z_]+\\.[a-z_<>]+)` \\|")
)

// retentionTables reads 03 section 4: every table with the class of its Retention line.
func retentionTables(t *testing.T) map[string]string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "docs", "spec", "03-data-model.md"))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	table := ""
	for _, line := range strings.Split(string(b), "\n") {
		if m := tableHeading.FindStringSubmatch(line); m != nil {
			table = m[1]
			continue
		}
		if strings.HasPrefix(line, "## ") {
			table = ""
		}
		if m := retentionOf.FindStringSubmatch(line); m != nil && table != "" {
			if _, seen := out[table]; !seen {
				out[table] = m[1]
			}
		}
	}
	if len(out) < 50 {
		t.Fatalf("read only %d tables with a Retention line from 03", len(out))
	}
	return out
}

// catalogueJobs reads the job kinds of the job catalogue of 02 12.3.
func catalogueJobs(t *testing.T) map[string]bool {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "docs", "spec", "02-architecture.md"))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, line := range strings.Split(string(b), "\n") {
		if m := jobRow.FindStringSubmatch(line); m != nil {
			out[m[1]] = true
		}
	}
	return out
}

// coverageProblems checks the rule of brief M7-05 design note 3 and returns one line per breach.
func coverageProblems(tables map[string]string, ps []retention.Participant, jobs map[string]bool) []string {
	var out []string
	owner := map[string]int{}
	for _, p := range ps {
		for _, tb := range p.Tables() {
			owner[tb]++
			class, ok := tables[tb]
			switch {
			case !ok:
				out = append(out, fmt.Sprintf("participant %T names %s, which 03 does not list", p, tb))
			case class != string(p.Class()):
				out = append(out, fmt.Sprintf("participant %T of class %s names %s of class %s", p, p.Class(), tb, class))
			}
		}
	}
	covered := func(tb string) bool {
		_, e := handledElsewhere[tb]
		_, w := withoutEndTime[tb]
		return owner[tb] > 0 || e || w
	}
	for tb, class := range tables {
		if !slices.Contains(timeClasses, class) {
			continue
		}
		_, e := handledElsewhere[tb]
		_, w := withoutEndTime[tb]
		parent, hasParent := withParent[tb]
		n := owner[tb]
		for _, b := range []bool{e, w, hasParent} {
			if b {
				n++
			}
		}
		switch {
		case n == 0:
			out = append(out, fmt.Sprintf("table %s of class %s has no participant and no listed owner", tb, class))
		case n > 1:
			out = append(out, fmt.Sprintf("table %s is covered %d times", tb, n))
		}
		for hasParent && !covered(parent) {
			next, ok := withParent[parent]
			if !ok {
				out = append(out, fmt.Sprintf("table %s goes with %s, which nothing covers", tb, parent))
				break
			}
			parent = next
		}
	}
	for _, list := range []map[string]string{handledElsewhere, withParent, withoutEndTime} {
		for tb, how := range list {
			if class, ok := tables[tb]; !ok || !slices.Contains(timeClasses, class) {
				out = append(out, fmt.Sprintf("listed table %s is no table of a time class of 03", tb))
			}
			if kind, ok := strings.CutPrefix(how, "job:"); ok && !jobs[kind] {
				out = append(out, fmt.Sprintf("table %s names job %s, which the catalogue of 02 lacks", tb, kind))
			}
		}
	}
	sort.Strings(out)
	return out
}

// TestEveryRetentionTableHasAParticipant (brief M7-05 design note 3): every table of 03 whose
// class ends by time is removed by exactly one participant of the jobs retention.<class>, goes
// with a covered parent, carries no end time, or is listed with the job, the module or the later
// ticket that owns it; a listed job must be in the catalogue of 02. A new table of 03 with such a
// class and none of these fails here.
func TestEveryRetentionTableHasAParticipant(t *testing.T) {
	t.Parallel()
	tables := retentionTables(t)
	ps := retentionParticipants(t)
	jobs := catalogueJobs(t)
	for _, c := range retention.JobClasses() {
		if !jobs[retention.JobKind(c)] && !jobs["retention.<class>"] {
			t.Errorf("job %s is not in the catalogue of 02", retention.JobKind(c))
		}
	}
	if problems := coverageProblems(tables, ps, jobs); len(problems) > 0 {
		t.Fatalf("retention coverage:\n%s", strings.Join(problems, "\n"))
	}
	t.Run("a new table fails", func(t *testing.T) {
		withNew := map[string]string{"example_tokens": "security"}
		for k, v := range tables {
			withNew[k] = v
		}
		got := coverageProblems(withNew, ps, jobs)
		if len(got) != 1 || !strings.Contains(got[0], "example_tokens") {
			t.Fatalf("problems %v, want one for example_tokens", got)
		}
	})
	t.Run("a job outside the catalogue fails", func(t *testing.T) {
		delete(jobs, "files.cleanup_pending")
		if got := coverageProblems(tables, ps, jobs); len(got) != 2 {
			t.Fatalf("problems %v, want the two upload tables", got)
		}
	})
}
