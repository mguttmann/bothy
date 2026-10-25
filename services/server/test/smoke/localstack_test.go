// SPDX-License-Identifier: Apache-2.0

package smoke

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// fakeLocalStack serves Loki, Prometheus and Tempo answers on one listener. Loki answers a full
// first page and a short second one, so the export must page; the canary sits in the last trace.
func fakeLocalStack(t *testing.T, canary string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		q := r.URL.Query()
		switch {
		case r.URL.Path == "/loki/api/v1/query_range":
			start, _ := strconv.ParseInt(q.Get("start"), 10, 64)
			n := lokiPageLimit
			if start >= 1_000_000_000_000_000_000+lokiPageLimit {
				n = 3
			}
			values := make([][]string, 0, n)
			for i := range n {
				values = append(values, []string{strconv.FormatInt(start+int64(i), 10), "line"})
			}
			ans := map[string]any{"data": map[string]any{"result": []any{
				map[string]any{"stream": map[string]string{"service_name": "bothy-server"}, "values": values},
			}}}
			_ = json.NewEncoder(w).Encode(ans)
		case r.URL.Path == "/api/v1/series":
			_, _ = fmt.Fprint(w, `{"data":[{"__name__":"http_server_request_duration_seconds_count","job":"bothy-server"},{"__name__":"up","job":"other"}]}`)
		case r.URL.Path == "/api/search":
			_, _ = fmt.Fprint(w, `{"traces":[{"traceID":"0a1b","rootServiceName":"bothy-server"},{"traceID":"2c3d","rootServiceName":"bothy-server"}]}`)
		case strings.HasPrefix(r.URL.Path, "/api/traces/"):
			id := strings.TrimPrefix(r.URL.Path, "/api/traces/")
			body := `{"batches":[]}`
			if id == "2c3d" {
				body = `{"batches":[{"attr":"x ` + canary + ` y"}]}`
			}
			_, _ = fmt.Fprint(w, body)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestExportLocal_PagesCountsAndFeedsTheScan(t *testing.T) {
	t.Parallel()
	canary := "smk0123456789abcdef01234567"
	srv := fakeLocalStack(t, canary)
	in := t.TempDir()
	if err := os.Mkdir(filepath.Join(in, "logs"), 0o700); err != nil {
		t.Fatal(err)
	}
	since := time.Unix(0, 1_000_000_000_000_000_000)
	ls := LocalStack{Loki: srv.URL, Prometheus: srv.URL, Tempo: srv.URL}
	n, err := ExportLocal(t.Context(), srv.Client(), ls, since, since.Add(time.Hour), "bothy-server", filepath.Join(in, "otel"))
	if err != nil {
		t.Fatalf("ExportLocal: %v", err)
	}
	if n.LogLines != lokiPageLimit+3 || n.Series != 1 || n.Traces != 2 {
		t.Errorf("counts %+v, want %d log lines over two pages, 1 series, 2 traces", n, lokiPageLimit+3)
	}
	hits, err := Scan(Canaries{{Label: LabelCanary, Value: canary}}, Roots(in))
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || !strings.HasSuffix(hits[0].File, "tempo-2c3d.json") {
		t.Errorf("hits %v, want the canary in the exported trace 2c3d", hits)
	}
}

func TestExportLocal_RefusesANonHexTraceID(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/search":
			_, _ = fmt.Fprint(w, `{"traces":[{"traceID":"../x"}]}`)
		case "/api/v1/series":
			_, _ = fmt.Fprint(w, `{"data":[]}`)
		default:
			_, _ = fmt.Fprint(w, `{"data":{"result":[]}}`)
		}
	}))
	defer srv.Close()
	_, err := ExportLocal(t.Context(), srv.Client(), LocalStack{Loki: srv.URL, Prometheus: srv.URL, Tempo: srv.URL},
		time.Unix(1, 0), time.Unix(2, 0), "bothy-server", t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "not hex") {
		t.Fatalf("ExportLocal = %v, want a refusal of the trace ID", err)
	}
}

func TestJobOf(t *testing.T) {
	t.Parallel()
	re := jobOf("bothy-server")
	for s, want := range map[string]bool{"bothy-server": true, "bothy/bothy-server": true, "bothy-server-x": false, "other": false} {
		if re.MatchString(s) != want {
			t.Errorf("jobOf(%q) = %v", s, !want)
		}
	}
}
