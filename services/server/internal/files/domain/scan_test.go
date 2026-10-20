// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"bytes"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files/domain"
)

func TestParseClamdReply(t *testing.T) {
	t.Parallel()
	ok := []struct {
		reply string
		want  domain.Verdict
	}{
		{"stream: OK", domain.Verdict{Kind: domain.VerdictClean}},
		{"stream: OK\x00", domain.Verdict{Kind: domain.VerdictClean}},
		{"stream: Win.Test.EICAR_HDB-1 FOUND", domain.Verdict{Kind: domain.VerdictInfected, Signature: "Win.Test.EICAR_HDB-1", Reason: domain.ReasonMalwareDetected}},
		{"stream: Heuristics.Limits.Exceeded.MaxRecursion FOUND\x00", domain.Verdict{Kind: domain.VerdictError, Signature: "Heuristics.Limits.Exceeded.MaxRecursion", Reason: domain.ReasonScanLimitsExceeded}},
		{"INSTREAM size limit exceeded. ERROR", domain.Verdict{Kind: domain.VerdictError, Reason: domain.ReasonScanStreamLimitExceeded}},
		{"stream: Can't allocate memory ERROR\x00", domain.Verdict{Kind: domain.VerdictError, Reason: domain.ReasonScanEngineError}},
	}
	for _, c := range ok {
		got, err := domain.ParseClamdReply([]byte(c.reply))
		if err != nil || got != c.want {
			t.Errorf("%q = %+v, %v; want %+v", c.reply, got, err, c.want)
		}
	}
	for _, bad := range []string{"", "\x00", "stream: ok", "stream:  FOUND", "stream: FOUND", "PONG", "stream: OK\n", "\x01\x02\xff", "stream: a b FOUND", "stream: OK\x00\x00"} {
		if v, err := domain.ParseClamdReply([]byte(bad)); !errors.Is(err, domain.ErrClamdProtocol) {
			t.Errorf("%q = %+v, %v; want ErrClamdProtocol", bad, v, err)
		}
	}
}

func FuzzParseClamdReply(f *testing.F) {
	for _, s := range []string{"stream: OK", "stream: OK\x00", "stream: X FOUND", "INSTREAM size limit exceeded. ERROR", "", "PONG"} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, reply []byte) {
		v, err := domain.ParseClamdReply(reply)
		if err == nil && v.Kind == domain.VerdictClean {
			if !bytes.Equal(reply, []byte("stream: OK")) && !bytes.Equal(reply, []byte("stream: OK\x00")) {
				t.Fatalf("%q parsed as clean", reply)
			}
		}
		if err == nil && v.Kind != domain.VerdictClean && v.Reason == "" {
			t.Fatalf("%q: a blocked verdict without reason", reply)
		}
	})
}

func TestDecidePass(t *testing.T) {
	t.Parallel()
	sum := [32]byte{1}
	other := [32]byte{2}
	base := domain.PassFacts{DeclaredSize: 10, CountedSize: 10, DeclaredSHA256: sum, ComputedSHA256: sum, Scanned: true,
		Verdict: domain.Verdict{Kind: domain.VerdictClean}}
	infected := domain.Verdict{Kind: domain.VerdictInfected, Signature: "X", Reason: domain.ReasonMalwareDetected}
	with := func(f func(*domain.PassFacts)) domain.PassFacts {
		p := base
		f(&p)
		return p
	}
	cases := []struct {
		name string
		in   domain.PassFacts
		want domain.PassOutcome
	}{
		{"size", with(func(p *domain.PassFacts) { p.CountedSize = 11 }),
			domain.PassOutcome{UploadState: "failed", ScanStatus: "pending", FailureReason: "size_mismatch", DeleteObjects: true}},
		{"checksum", with(func(p *domain.PassFacts) { p.ComputedSHA256 = other }),
			domain.PassOutcome{UploadState: "failed", ScanStatus: "pending", FailureReason: "checksum_mismatch", DeleteObjects: true}},
		{"denied", with(func(p *domain.PassFacts) { p.TypeDenied = true }),
			domain.PassOutcome{UploadState: "failed", ScanStatus: "pending", FailureReason: "file_type_denied", SetMIMEType: true, DeleteObjects: true}},
		{"off", with(func(p *domain.PassFacts) { p.ScanOff, p.Scanned = true, false }),
			domain.PassOutcome{UploadState: "complete", ScanStatus: "skipped", SetMIMEType: true, MetricOutcome: "skipped"}},
		{"clean", base, domain.PassOutcome{UploadState: "complete", ScanStatus: "clean", SetMIMEType: true, ScanEngine: true, MetricOutcome: "clean"}},
		{"limits", with(func(p *domain.PassFacts) {
			p.Verdict = domain.Verdict{Kind: domain.VerdictError, Signature: "Heuristics.Limits.Exceeded.MaxScanSize", Reason: "scan_limits_exceeded"}
		}), domain.PassOutcome{UploadState: "complete", ScanStatus: "error", FailureReason: "scan_limits_exceeded", SetMIMEType: true, DeleteObjects: true, ScanEngine: true, MetricOutcome: "error"}},
		{"infected", with(func(p *domain.PassFacts) { p.Verdict = infected }),
			domain.PassOutcome{UploadState: "complete", ScanStatus: "infected", FailureReason: "malware_detected", SetMIMEType: true, DeleteObjects: true, AuditMalware: true, ScanEngine: true, MetricOutcome: "infected"}},
		{"stream limit", with(func(p *domain.PassFacts) {
			p.Verdict = domain.Verdict{Kind: domain.VerdictError, Reason: "scan_stream_limit_exceeded"}
		}), domain.PassOutcome{UploadState: "complete", ScanStatus: "error", FailureReason: "scan_stream_limit_exceeded", SetMIMEType: true, DeleteObjects: true, ScanEngine: true, MetricOutcome: "error"}},
		{"engine error", with(func(p *domain.PassFacts) {
			p.Verdict = domain.Verdict{Kind: domain.VerdictError, Reason: "scan_engine_error"}
		}), domain.PassOutcome{UploadState: "complete", ScanStatus: "error", FailureReason: "scan_engine_error", SetMIMEType: true, DeleteObjects: true, ScanEngine: true, MetricOutcome: "error"}},
		{"checksum with FOUND", with(func(p *domain.PassFacts) { p.ComputedSHA256, p.Verdict = other, infected }),
			domain.PassOutcome{UploadState: "failed", ScanStatus: "pending", FailureReason: "checksum_mismatch", DeleteObjects: true, AuditMalware: true}},
		{"size with transient", with(func(p *domain.PassFacts) { p.CountedSize, p.Scanned = 9, false }),
			domain.PassOutcome{UploadState: "failed", ScanStatus: "pending", FailureReason: "size_mismatch", DeleteObjects: true}},
		{"denied with transient", with(func(p *domain.PassFacts) { p.TypeDenied, p.Scanned = true, false }),
			domain.PassOutcome{UploadState: "failed", ScanStatus: "pending", FailureReason: "file_type_denied", SetMIMEType: true, DeleteObjects: true}},
		{"transient", with(func(p *domain.PassFacts) { p.Scanned = false }), domain.PassOutcome{Retry: true}},
	}
	for _, c := range cases {
		if got := domain.DecidePass(c.in); got != c.want {
			t.Errorf("%s: %+v, want %+v", c.name, got, c.want)
		}
	}
}

func TestNextScanDelay(t *testing.T) {
	t.Parallel()
	first, ceiling := 30*time.Second, 5*time.Minute
	want := []time.Duration{30 * time.Second, time.Minute, 2 * time.Minute, 4 * time.Minute, 5 * time.Minute, 5 * time.Minute}
	for r, w := range want {
		if got := domain.NextScanDelay(int64(r), first, ceiling); got != w {
			t.Errorf("round %d: %v, want %v", r, got, w)
		}
	}
	if got := domain.NextScanDelay(math.MaxInt64, first, ceiling); got != ceiling {
		t.Errorf("largest round: %v", got)
	}
	if got := domain.NextScanDelay(math.MaxInt64, time.Nanosecond, time.Duration(math.MaxInt64)); got <= 0 || got > time.Duration(math.MaxInt64) {
		t.Errorf("overflow: %v", got)
	}
	if got := domain.NextScanDelay(-1, first, ceiling); got != first {
		t.Errorf("negative round: %v", got)
	}
}

func TestDeliverableAndFinal(t *testing.T) {
	t.Parallel()
	cases := []struct {
		up, scan             string
		deleted              bool
		deliverable, isFinal bool
	}{
		{"pending", "pending", false, false, false},
		{"failed", "pending", false, false, true},
		{"complete", "clean", false, true, true},
		{"complete", "skipped", false, true, true},
		{"complete", "infected", false, false, true},
		{"complete", "error", false, false, true},
		{"complete", "clean", true, false, false},
		{"complete", "pending", false, false, false},
	}
	for _, c := range cases {
		if got := domain.IsDeliverable(c.up, c.scan, c.deleted); got != c.deliverable {
			t.Errorf("IsDeliverable(%s,%s,%v) = %v", c.up, c.scan, c.deleted, got)
		}
		if got := domain.Final(c.up, c.scan, c.deleted); got != c.isFinal {
			t.Errorf("Final(%s,%s,%v) = %v", c.up, c.scan, c.deleted, got)
		}
	}
}

func TestAuditSignature(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"Win.Test.EICAR_HDB-1":        "win.test.eicar_hdb-1",
		"Local.Test.EICAR.UNOFFICIAL": "local.test.eicar.unofficial",
		"Some Name/With Slash":        "some_name_with_slash",
		"":                            "unknown",
		"_lead":                       "x_lead",
	}
	for in, want := range cases {
		if got := domain.AuditSignature(in); got != want {
			t.Errorf("%q = %q, want %q", in, got, want)
		}
	}
	long := domain.AuditSignature(strings.Repeat("A", 200))
	if len(long) != domain.AuditCodeMaxBytes || strings.Trim(long, "a") != "" {
		t.Errorf("long name = %q", long)
	}
}

func TestMediaTypeAndDenied(t *testing.T) {
	t.Parallel()
	if got := domain.MediaType(" Text/Plain; charset=utf-8"); got != "text/plain" {
		t.Errorf("MediaType = %q", got)
	}
	if !domain.TypeDenied("image/png", []string{"", "IMAGE/PNG; x=1"}) || domain.TypeDenied("image/png", []string{"image/jpeg", ""}) {
		t.Error("TypeDenied")
	}
}
