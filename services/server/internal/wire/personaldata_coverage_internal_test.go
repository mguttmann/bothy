// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	chatapp "github.com/OWNER/bothy/services/server/internal/chat/app"
	filesapp "github.com/OWNER/bothy/services/server/internal/files/app"
	identityapp "github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/personaldata"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	messagingapp "github.com/OWNER/bothy/services/server/internal/messaging/app"
)

// personalDataExceptions are the tables of 20 section 2 with personal data that no participant
// names, each with the reason (ruling D-M7-06-6). A table listed here must not also have a
// participant.
var personalDataExceptions = map[string]string{
	"dev_environments": "the table comes with V2 (03, 21) and has no migration yet; its module adds the participant with it",
}

// tableCell matches the table names of the first cell of a row of 20 section 2.
var tableCell = regexp.MustCompile("`([a-z_]+)`")

// personalTables reads 20 section 2: every table whose personal reference is not "no".
func personalTables(t *testing.T) []string {
	t.Helper()
	f, err := os.Open(filepath.Join("..", "..", "..", "..", "docs", "spec", "20-privacy.md")) // #nosec G304 -- a fixed path of the repository
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	var out []string
	in := false
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "## ") {
			in = strings.HasPrefix(line, "## 2. ")
			continue
		}
		if !in || !strings.HasPrefix(line, "| `") {
			continue
		}
		cells := strings.Split(line, "|")
		if len(cells) < 4 || strings.TrimSpace(cells[2]) == "no" {
			continue
		}
		for _, m := range tableCell.FindAllStringSubmatch(cells[1], -1) {
			out = append(out, m[1])
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if len(out) < 40 {
		t.Fatalf("20 section 2 lists only %d tables with personal data", len(out))
	}
	return out
}

// moduleTables reads the tables of every module of architecture/modules.yaml.
func moduleTables(t *testing.T) map[string][]string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "architecture", "modules.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var reg struct {
		Modules map[string]struct {
			Tables []string `yaml:"tables"`
		} `yaml:"modules"`
	}
	if err := yaml.Unmarshal(data, &reg); err != nil {
		t.Fatal(err)
	}
	out := map[string][]string{}
	for name, m := range reg.Modules {
		out[name] = m.Tables
	}
	return out
}

// noUnit and noEvents are collaborators the coverage test never calls.
type (
	noUnit   struct{}
	noEvents struct{}
)

func (noUnit) Do(context.Context, func(context.Context) error, ...txn.Option) error { return nil }
func (noEvents) Append(context.Context, ...eventlog.Event) error                    { return nil }

// TestEveryPersonalTableHasAParticipant (brief M7-06 item 17): every table of 20 section 2 with
// personal data is named by exactly one participant that internal/wire registers, or listed once
// in personalDataExceptions with its reason; every table a participant names is a table of its own
// module in architecture/modules.yaml and has personal data in 20 section 2.
func TestEveryPersonalTableHasAParticipant(t *testing.T) {
	t.Parallel()
	list, err := personalDataList(personalDataModules{
		uow: noUnit{}, events: noEvents{}, chat: &Chat{Service: new(chatapp.Service)},
		messaging: &Messaging{Service: new(messagingapp.Service)}, files: &Files{Service: new(filesapp.Service)},
		identity: &Identity{Lifecycle: new(identityapp.Lifecycle)},
	})
	if err != nil {
		t.Fatal(err)
	}
	reg := personaldata.NewRegistry()
	for _, p := range list {
		if err := reg.Register(p); err != nil {
			t.Fatal(err)
		}
	}
	personal := personalTables(t)
	owned := moduleTables(t)
	covered := map[string]string{}
	for _, p := range reg.All() {
		for _, table := range p.Tables() {
			covered[table] = p.Module()
			if !slices.Contains(owned[p.Module()], table) {
				t.Errorf("participant %s names %s, which is no table of its module", p.Module(), table)
			}
			if !slices.Contains(personal, table) {
				t.Errorf("participant %s names %s, which has no personal data in 20 section 2", p.Module(), table)
			}
		}
	}
	for _, table := range personal {
		_, byParticipant := covered[table]
		_, excepted := personalDataExceptions[table]
		switch {
		case byParticipant && excepted:
			t.Errorf("%s has a participant and an exception", table)
		case !byParticipant && !excepted:
			t.Errorf("%s has personal data in 20 section 2 but no participant", table)
		}
	}
	for table := range personalDataExceptions {
		if !slices.Contains(personal, table) {
			t.Errorf("the exception %s is no table of 20 section 2 with personal data", table)
		}
	}
	// Notify before identity: push registrations hold the devices identity deletes in step 2.
	order := map[string]int{}
	for i, p := range reg.All() {
		order[p.Module()] = i
	}
	if order["notify"] > order["identity"] || order["identity"] != len(reg.All())-1 {
		t.Errorf("the order of the participants is wrong: %v", order)
	}
}
