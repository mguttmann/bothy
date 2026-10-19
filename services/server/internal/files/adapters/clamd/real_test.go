// SPDX-License-Identifier: Apache-2.0

package clamd_test

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/clamd"
	"github.com/OWNER/bothy/services/server/internal/files/adapters/clamd/clamdtest"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
)

// The limits of the shared test clamd; small, so the limit tests stay fast.
const (
	testStreamMax    = 256 << 10
	testMaxScanSize  = 128 << 10
	testMaxRecursion = 3
)

var shared struct {
	once sync.Once
	srv  *clamdtest.Clamd
	err  error
}

// TestMain stops the shared clamd after the tests. Without a Docker-compatible runtime the real
// clamd tests fail; they never skip (plan P-09).
func TestMain(m *testing.M) {
	code := m.Run()
	if shared.srv != nil {
		if err := shared.srv.Terminate(context.Background()); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "clamd tests: %v\n", err)
			if code == 0 {
				code = 1
			}
		}
	}
	os.Exit(code)
}

// realClamd returns the shared test clamd, started on first use.
func realClamd(t *testing.T) *clamdtest.Clamd {
	t.Helper()
	shared.once.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		shared.srv, shared.err = clamdtest.Start(ctx, clamdtest.Options{StreamMaxLength: testStreamMax,
			MaxScanSize: testMaxScanSize, MaxFileSize: testStreamMax, MaxRecursion: testMaxRecursion})
	})
	if shared.err != nil {
		t.Fatalf("clamd: %v", shared.err)
	}
	return shared.srv
}

func realClient(t *testing.T, addr string) *clamd.Client {
	t.Helper()
	return newClient(t, addr, 4096, 4)
}

// nestedZip wraps payload in depth levels of ZIP archives.
func nestedZip(t *testing.T, payload []byte, depth int) []byte {
	t.Helper()
	b := payload
	for i := range depth {
		var buf bytes.Buffer
		w := zip.NewWriter(&buf)
		f, err := w.Create(fmt.Sprintf("level-%d.bin", i))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write(b); err != nil {
			t.Fatal(err)
		}
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		b = buf.Bytes()
	}
	return b
}

func TestRealClamdVerdicts(t *testing.T) {
	srv := realClamd(t)
	c := realClient(t, srv.Address())
	ctx := t.Context()

	if err := c.Ping(ctx); err != nil {
		t.Fatalf("Ping: %v", err)
	}
	v, err := c.Scan(ctx, strings.NewReader("a harmless text file of Alex\n"))
	if err != nil || v.Kind != domain.VerdictClean {
		t.Errorf("clean: %+v, %v", v, err)
	}
	v, err = c.Scan(ctx, bytes.NewReader(clamdtest.EICAR()))
	if err != nil || v.Kind != domain.VerdictInfected || !strings.HasPrefix(v.Signature, clamdtest.SignatureName) {
		t.Errorf("EICAR: %+v, %v", v, err)
	}
	v, err = c.Scan(ctx, bytes.NewReader(nestedZip(t, []byte("plain"), testMaxRecursion+2)))
	if err != nil || v.Kind != domain.VerdictError || v.Reason != domain.ReasonScanLimitsExceeded {
		t.Errorf("nested archive: %+v, %v", v, err)
	}
	// A small archive whose content expands beyond MaxScanSize.
	big := nestedZip(t, bytes.Repeat([]byte{0}, testMaxScanSize*4), 1)
	if len(big) >= testMaxScanSize {
		t.Fatalf("the archive has %d bytes and is not small", len(big))
	}
	v, err = c.Scan(ctx, bytes.NewReader(big))
	if err != nil || v.Kind != domain.VerdictError || v.Reason != domain.ReasonScanLimitsExceeded {
		t.Errorf("archive over MaxScanSize: %+v, %v", v, err)
	}
	v, err = c.Scan(ctx, bytes.NewReader(bytes.Repeat([]byte("y"), testStreamMax*2)))
	if err != nil || v.Kind != domain.VerdictError || v.Reason != domain.ReasonScanStreamLimitExceeded {
		t.Errorf("stream limit: %+v, %v", v, err)
	}
}

func TestRealClamdStopAndStart(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 6*time.Minute)
	defer cancel()
	srv, err := clamdtest.Start(ctx, clamdtest.Options{Restartable: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Terminate(context.Background()) })
	c := realClient(t, srv.Address())
	if err := c.Ping(ctx); err != nil {
		t.Fatalf("Ping: %v", err)
	}
	if err := srv.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	if err := c.Ping(ctx); err == nil {
		t.Fatal("Ping of a stopped clamd succeeded")
	}
	if _, err := c.Scan(ctx, strings.NewReader("x")); err == nil || errors.Is(err, domain.ErrClamdProtocol) {
		t.Fatalf("Scan of a stopped clamd = %v, want a transport error", err)
	}
	if err := srv.StartAgain(ctx); err != nil {
		t.Fatal(err)
	}
	if err := c.Ping(ctx); err != nil {
		t.Fatalf("Ping after the start: %v", err)
	}
}

// TestClamdImagePinned holds the image of clamdtest equal to the Compose stack: the repository of
// the service clamav, a version and a sha256 digest, and the version of BOTHY_CLAMAV_VERSION.
func TestClamdImagePinned(t *testing.T) {
	t.Parallel()
	m := regexp.MustCompile(`^([a-z0-9][a-z0-9./_-]*):([^@:]+)@sha256:[0-9a-f]{64}$`).FindStringSubmatch(clamdtest.Image)
	if m == nil {
		t.Fatalf("Image %q is not <repository>:<version>@sha256:<digest>", clamdtest.Image)
	}
	repo, version := m[1], m[2]
	root := filepath.Join("..", "..", "..", "..", "..", "..")
	read := func(rel string) string {
		b, err := os.ReadFile(filepath.Join(root, rel)) // #nosec G304 -- a fixed file of the repository
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	if want := "image: " + repo + ":${BOTHY_CLAMAV_VERSION:-" + version + "}"; !strings.Contains(read("infra/compose/compose.yaml"), want) {
		t.Errorf("infra/compose/compose.yaml has no line %q", want)
	}
	if !regexp.MustCompile(`(?m)^# renovate: datasource=docker depName=` + regexp.QuoteMeta(repo) + `\nBOTHY_CLAMAV_VERSION=` + regexp.QuoteMeta(version) + `$`).MatchString(read("infra/compose/.env.example")) {
		t.Errorf("infra/compose/.env.example does not set BOTHY_CLAMAV_VERSION=%s under its renovate comment", version)
	}
	if !strings.Contains(read("services/server/internal/files/adapters/clamd/clamdtest/container.go"), "// renovate: datasource=docker depName="+repo+"\nconst Image = ") {
		t.Error("the renovate comment does not stand directly above Image")
	}
}
