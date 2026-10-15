// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// seq returns n bytes counting up from start.
func seq(start byte, n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = start + byte(i)
	}
	return out
}

// fingerprintVectors is testdata/fingerprint_vectors.json.
type fingerprintVectors struct {
	Vectors []struct {
		Noise       string `json:"noise_public"`
		Auth        string `json:"ed25519_public"`
		Fingerprint string `json:"fingerprint"`
	} `json:"vectors"`
}

// readVectors reads a vector file of the form of services/host/testdata/noise/fingerprint.json.
func readVectors(t *testing.T, path string) fingerprintVectors {
	t.Helper()
	raw, err := os.ReadFile(path) //nolint:gosec // G304: a test data path of this package
	if err != nil {
		t.Fatal(err)
	}
	var v fingerprintVectors
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	if len(v.Vectors) == 0 {
		t.Fatalf("%s holds no vector", path)
	}
	return v
}

// checkVectors computes every vector of v.
func checkVectors(t *testing.T, v fingerprintVectors) {
	t.Helper()
	for _, c := range v.Vectors {
		noise, err1 := hex.DecodeString(c.Noise)
		auth, err2 := hex.DecodeString(c.Auth)
		if err1 != nil || err2 != nil {
			t.Fatalf("vector %s: bad hex", c.Fingerprint)
		}
		got, err := domain.ComputeFingerprint(noise, auth)
		if err != nil || got != c.Fingerprint {
			t.Errorf("fingerprint %q, %v; want %q", got, err, c.Fingerprint)
		}
	}
}

// TestFingerprint_KnownVector (24 section 4.3, Design note 5): the independent vector of the brief.
func TestFingerprint_KnownVector(t *testing.T) {
	t.Parallel()
	got, err := domain.ComputeFingerprint(seq(0x00, 32), seq(0x20, 32))
	if err != nil {
		t.Fatal(err)
	}
	const want = "F0CC-6QS6-RF5D-3BSH-CAZQ-PXMR-J8RA-AHK5-1HF0-9S02-1GW6-BW3J-HK90"
	if got != want {
		t.Fatalf("fingerprint %s, want %s", got, want)
	}
	if c := domain.CompactFingerprint(got); c != strings.ReplaceAll(want, "-", "") || len(c) != domain.FingerprintChars {
		t.Fatalf("compact form %s", c)
	}
	checkVectors(t, readVectors(t, filepath.Join("testdata", "fingerprint_vectors.json")))
}

// TestFingerprint_HostVectors: the server computes every vector of the host service (M2-38), so
// both sides show the same fingerprint.
func TestFingerprint_HostVectors(t *testing.T) {
	t.Parallel()
	checkVectors(t, readVectors(t, filepath.Join("..", "..", "..", "..", "host", "testdata", "noise", "fingerprint.json")))
}

// TestFingerprint_CrockfordTable: the RFC 4648 test inputs in the Crockford alphabet.
func TestFingerprint_CrockfordTable(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"": "", "f": "CR", "fo": "CSQG", "foo": "CSQPY", "foob": "CSQPYRG", "fooba": "CSQPYRK1", "foobar": "CSQPYRK1E8",
	} {
		if got := domain.CrockfordForTest([]byte(in)); got != want {
			t.Errorf("crockford(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestFingerprint_RejectsWrongLength: both keys must be exactly 32 bytes.
func TestFingerprint_RejectsWrongLength(t *testing.T) {
	t.Parallel()
	for _, c := range [][2][]byte{{seq(0, 31), seq(0, 32)}, {seq(0, 32), seq(0, 33)}, {nil, seq(0, 32)}, {seq(0, 32), nil}} {
		if _, err := domain.ComputeFingerprint(c[0], c[1]); err == nil {
			t.Errorf("keys of %d and %d bytes accepted", len(c[0]), len(c[1]))
		}
	}
}

// TestFingerprint_KeyOrderMatters: swapping the keys gives another fingerprint.
func TestFingerprint_KeyOrderMatters(t *testing.T) {
	t.Parallel()
	a, _ := domain.ComputeFingerprint(seq(0x00, 32), seq(0x20, 32))
	b, _ := domain.ComputeFingerprint(seq(0x20, 32), seq(0x00, 32))
	if a == b {
		t.Fatal("the key order does not matter")
	}
}
