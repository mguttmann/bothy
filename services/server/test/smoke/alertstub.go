// SPDX-License-Identifier: Apache-2.0

package smoke

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

// Paths and files of the alert stub of the end-to-end test (brief M7-20 Design note 2).
const (
	// StubFlagFile is the name of the file in the control directory whose presence makes the
	// metric of the test rule 1; the probe removes it to let the test alerts resolve.
	StubFlagFile = "firing"
	// StubMetric is the gauge the test rule of infra/compose/test/alert-e2e/rules.yaml reads.
	StubMetric = "alert_path_e2e_firing"
	// stubMaxBody bounds one recorded notification; Alertmanager sends a few kilobytes.
	stubMaxBody = 1 << 20
)

// stubReceivers maps the paths the stub accepts to the files it appends to: the external on-call
// channel and the heartbeat service, each a stand-in for a service of the operator's choice.
var stubReceivers = map[string]string{
	"/oncall":    "oncall.jsonl",
	"/heartbeat": "heartbeat.jsonl",
}

// AlertStub is the HTTP stand-in for the external receivers of Alertmanager and the scrape target
// of the test rule. POST /oncall and POST /heartbeat append the JSON body as one line to
// oncall.jsonl and heartbeat.jsonl in Out; GET /metrics reports StubMetric 1 while the file
// StubFlagFile exists in Ctl, else 0. Everything else is 404. It is safe for concurrent use.
//
// Example:
//
//	srv := &http.Server{Addr: ":9000", Handler: &smoke.AlertStub{Out: "/out/stub", Ctl: "/ctl"}}
type AlertStub struct {
	Out, Ctl string

	mu sync.Mutex
}

// ServeHTTP implements http.Handler.
func (s *AlertStub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && r.URL.Path == "/metrics" {
		s.metrics(w)
		return
	}
	file, ok := stubReceivers[r.URL.Path]
	if !ok || r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, stubMaxBody+1))
	if err != nil || len(body) > stubMaxBody {
		http.Error(w, "body too large or unreadable", http.StatusBadRequest)
		return
	}
	var line bytes.Buffer
	if err := json.Compact(&line, body); err != nil {
		http.Error(w, "body is not JSON", http.StatusBadRequest)
		return
	}
	line.WriteByte('\n')
	if err := s.appendLine(file, line.Bytes()); err != nil {
		http.Error(w, "cannot record", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *AlertStub) metrics(w http.ResponseWriter) {
	v := 0
	if _, err := os.Stat(filepath.Join(s.Ctl, StubFlagFile)); err == nil {
		v = 1
	} else if !errors.Is(err, fs.ErrNotExist) {
		http.Error(w, "cannot read the control directory", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	_, _ = fmt.Fprintf(w, "# HELP %s 1 while the test alerts of the alert path test fire.\n# TYPE %s gauge\n%s %d\n", StubMetric, StubMetric, StubMetric, v)
}

func (s *AlertStub) appendLine(file string, line []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := os.OpenFile(filepath.Join(s.Out, file), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) // #nosec G304 -- a fixed name in the output directory of the run
	if err != nil {
		return err
	}
	_, werr := f.Write(line)
	return errors.Join(werr, f.Close())
}
