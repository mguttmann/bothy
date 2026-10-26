// SPDX-License-Identifier: Apache-2.0

package smoke

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"
)

// Bounds of the export of the local stack (overlay local, docs/observability/backends.md).
const (
	// lokiPageLimit is the number of log lines per query_range page.
	lokiPageLimit = 5000
	// maxLokiPages bounds the pages of one export; a test run writes far fewer lines.
	maxLokiPages = 200
	// tempoSearchLimit is the number of traces one search returns; every one is fetched.
	tempoSearchLimit = 1000
	// maxExportBody bounds one answer of the local stack.
	maxExportBody = 256 << 20
)

// LocalStack names the query addresses of the local default stack of the overlay local: Loki,
// Prometheus and Tempo inside the Compose network internal (for example http://loki:3100).
type LocalStack struct {
	Loki, Prometheus, Tempo string
}

// TelemetryCounts is what the export found per signal for the service of the server.
type TelemetryCounts struct {
	LogLines, Series, Traces int
}

// ExportLocal reads everything the local stack received since `since` into files below dir:
// every log line of Loki (query_range over every stream, page by page), every series of
// Prometheus with its labels and every trace of Tempo the search lists, each trace in full. The
// files are the input of Scan, so a canary in any exported value is found. It returns how many log
// lines, series and traces belong to service; the caller requires all three, so the export proves
// that telemetry arrived through the overlay. The error names the backend and never a value.
func ExportLocal(ctx context.Context, hc *http.Client, ls LocalStack, since, until time.Time, service, dir string) (TelemetryCounts, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return TelemetryCounts{}, err
	}
	var n TelemetryCounts
	var err error
	if n.LogLines, err = exportLoki(ctx, hc, ls.Loki, since, until, service, dir); err != nil {
		return n, fmt.Errorf("smoke: loki: %w", err)
	}
	if n.Series, err = exportPrometheus(ctx, hc, ls.Prometheus, since, until, service, dir); err != nil {
		return n, fmt.Errorf("smoke: prometheus: %w", err)
	}
	if n.Traces, err = exportTempo(ctx, hc, ls.Tempo, since, until, service, dir); err != nil {
		return n, fmt.Errorf("smoke: tempo: %w", err)
	}
	return n, nil
}

// getJSON fetches target and returns the body; a non-200 answer is an error with the status.
func getJSON(ctx context.Context, hc *http.Client, target string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxExportBody))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s answered %d", req.URL.Path, resp.StatusCode)
	}
	return b, nil
}

// lokiAnswer is the part of a query_range answer of streams the export reads.
type lokiAnswer struct {
	Data struct {
		Result []struct {
			Stream map[string]string `json:"stream"`
			Values [][]string        `json:"values"`
		} `json:"result"`
	} `json:"data"`
}

// exportLoki pages through query_range forward in time; each page is one file.
func exportLoki(ctx context.Context, hc *http.Client, base string, since, until time.Time, service, dir string) (int, error) {
	start, own := since.UnixNano(), 0
	for page := 0; ; page++ {
		if page == maxLokiPages {
			return own, fmt.Errorf("more than %d pages", maxLokiPages)
		}
		q := url.Values{
			"query": {`{service_name=~".+"}`}, "direction": {"forward"}, "limit": {strconv.Itoa(lokiPageLimit)},
			"start": {strconv.FormatInt(start, 10)}, "end": {strconv.FormatInt(until.UnixNano(), 10)},
		}
		b, err := getJSON(ctx, hc, base+"/loki/api/v1/query_range?"+q.Encode())
		if err != nil {
			return own, err
		}
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("loki-%03d.json", page)), b, 0o600); err != nil {
			return own, err
		}
		var a lokiAnswer
		if err := json.Unmarshal(b, &a); err != nil {
			return own, fmt.Errorf("decode: %w", err)
		}
		lines, last := 0, start
		for _, s := range a.Data.Result {
			for _, v := range s.Values {
				if len(v) == 0 {
					continue
				}
				lines++
				if s.Stream["service_name"] == service {
					own++
				}
				if ts, err := strconv.ParseInt(v[0], 10, 64); err == nil && ts > last {
					last = ts
				}
			}
		}
		if lines < lokiPageLimit {
			return own, nil
		}
		// The next page starts after the newest line of this one; lines of the same
		// nanosecond on the border are rare and only scanned once less.
		start = last + 1
	}
}

// jobOf matches the job label of a series of service (job is service.name, or
// service.namespace/service.name, after the OTLP translation of Prometheus).
func jobOf(service string) *regexp.Regexp {
	return regexp.MustCompile(`^(.+/)?` + regexp.QuoteMeta(service) + `$`)
}

// exportPrometheus writes every series of the time range with all its labels.
func exportPrometheus(ctx context.Context, hc *http.Client, base string, since, until time.Time, service, dir string) (int, error) {
	q := url.Values{"match[]": {`{__name__=~".+"}`}, "start": {strconv.FormatInt(since.Unix(), 10)}, "end": {strconv.FormatInt(until.Unix(), 10)}}
	b, err := getJSON(ctx, hc, base+"/api/v1/series?"+q.Encode())
	if err != nil {
		return 0, err
	}
	if err := os.WriteFile(filepath.Join(dir, "prometheus-series.json"), b, 0o600); err != nil {
		return 0, err
	}
	var a struct {
		Data []map[string]string `json:"data"`
	}
	if err := json.Unmarshal(b, &a); err != nil {
		return 0, fmt.Errorf("decode: %w", err)
	}
	job, own := jobOf(service), 0
	for _, s := range a.Data {
		if job.MatchString(s["job"]) {
			own++
		}
	}
	return own, nil
}

// exportTempo lists the traces of the time range and writes each one in full.
func exportTempo(ctx context.Context, hc *http.Client, base string, since, until time.Time, service, dir string) (int, error) {
	q := url.Values{"start": {strconv.FormatInt(since.Unix(), 10)}, "end": {strconv.FormatInt(until.Unix(), 10)}, "limit": {strconv.Itoa(tempoSearchLimit)}}
	b, err := getJSON(ctx, hc, base+"/api/search?"+q.Encode())
	if err != nil {
		return 0, err
	}
	if err := os.WriteFile(filepath.Join(dir, "tempo-search.json"), b, 0o600); err != nil {
		return 0, err
	}
	var a struct {
		Traces []struct {
			TraceID         string `json:"traceID"`
			RootServiceName string `json:"rootServiceName"`
		} `json:"traces"`
	}
	if err := json.Unmarshal(b, &a); err != nil {
		return 0, fmt.Errorf("decode: %w", err)
	}
	if len(a.Traces) >= tempoSearchLimit {
		return 0, fmt.Errorf("the search hit its limit of %d traces; some would stay unscanned", tempoSearchLimit)
	}
	own := 0
	hexID := regexp.MustCompile(`^[0-9a-fA-F]{1,32}$`)
	for _, t := range a.Traces {
		if !hexID.MatchString(t.TraceID) {
			return own, errors.New("the search returned a trace ID that is not hex")
		}
		tb, err := getJSON(ctx, hc, base+"/api/traces/"+t.TraceID)
		if err != nil {
			return own, err
		}
		if err := os.WriteFile(filepath.Join(dir, "tempo-"+t.TraceID+".json"), tb, 0o600); err != nil {
			return own, err
		}
		if t.RootServiceName == service {
			own++
		}
	}
	return own, nil
}
