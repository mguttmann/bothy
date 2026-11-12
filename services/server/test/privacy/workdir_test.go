// SPDX-License-Identifier: Apache-2.0

package privacy_test

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/personaldata"
)

// workDirProbe is a participant that records, each time an export calls it, the entries of dir.
type workDirProbe struct {
	dir string

	mu      sync.Mutex
	entries []string
	err     error
}

func (*workDirProbe) Module() string   { return "work_dir_probe" }
func (*workDirProbe) Tables() []string { return []string{"work_dir_probe_test_table"} }

func (p *workDirProbe) Export(context.Context, personaldata.Export, personaldata.Writer) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	list, err := os.ReadDir(p.dir)
	if err != nil {
		p.err = err
		return nil
	}
	for _, e := range list {
		p.entries = append(p.entries, e.Name())
	}
	return nil
}

func (*workDirProbe) Erase(context.Context, personaldata.Step, personaldata.Erasure) error {
	return nil
}

// TestExportBuildsInWorkDir (20 section 5.2): with privacy.export_work_dir the export package is
// built in a directory of its own below that directory, not in the temporary directory of the
// system, and nothing of it stays there after the export.
func TestExportBuildsInWorkDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	probe := &workDirProbe{dir: dir}
	w := newWorld(t, map[string]string{"privacy.export_work_dir": dir},
		func(ps []personaldata.Participant) []personaldata.Participant { return append(ps, probe) })
	_, raw := w.export(w.w.People["Alex"])
	if files, _ := unzip(t, raw); files["manifest.json"] == nil {
		t.Fatal("the package has no manifest.json")
	}
	probe.mu.Lock()
	during, err := probe.entries, probe.err
	probe.mu.Unlock()
	if err != nil || len(during) != 1 || !strings.HasPrefix(during[0], "bothy-export-") {
		t.Fatalf("privacy.export_work_dir during the export holds %v (%v), want one directory bothy-export-*", during, err)
	}
	if left, err := os.ReadDir(dir); err != nil || len(left) != 0 {
		t.Fatalf("privacy.export_work_dir after the export holds %d entries (%v), want none", len(left), err)
	}
}
