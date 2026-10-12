// SPDX-License-Identifier: Apache-2.0

package fingerprint

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestEncodeRFC4648Vectors(t *testing.T) {
	// The inputs of RFC 4648 section 10, encoded with the Crockford alphabet.
	cases := map[string]string{
		"":       "",
		"f":      "CR",
		"fo":     "CSQG",
		"foo":    "CSQPY",
		"foob":   "CSQPYRG",
		"fooba":  "CSQPYRK1",
		"foobar": "CSQPYRK1E8",
	}
	for in, want := range cases {
		if got := Encode([]byte(in)); got != want {
			t.Errorf("Encode(%q) = %q, want %q", in, got, want)
		}
	}
}

func sequence(from byte) []byte {
	b := make([]byte, 32)
	for i := range b {
		b[i] = from + byte(i)
	}
	return b
}

func TestComputeKnownVector(t *testing.T) {
	got, err := Compute(sequence(0x00), sequence(0x20))
	if err != nil {
		t.Fatal(err)
	}
	const want = "F0CC-6QS6-RF5D-3BSH-CAZQ-PXMR-J8RA-AHK5-1HF0-9S02-1GW6-BW3J-HK90"
	if got != want {
		t.Errorf("Compute = %q, want %q", got, want)
	}
}

func TestComputeRejectsWrongLength(t *testing.T) {
	for _, n := range []int{0, 31, 33} {
		if _, err := Compute(make([]byte, n), make([]byte, 32)); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("noise key of %d bytes: err = %v, want ErrInvalidKey", n, err)
		}
		if _, err := Compute(make([]byte, 32), make([]byte, n)); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("auth key of %d bytes: err = %v, want ErrInvalidKey", n, err)
		}
	}
}

func TestDisplayForm(t *testing.T) {
	got, err := Compute(sequence(0x40), sequence(0x60))
	if err != nil {
		t.Fatal(err)
	}
	groups := strings.Split(got, "-")
	if len(got) != 64 || len(groups) != 13 {
		t.Fatalf("display form has %d characters in %d groups, want 64 in 13", len(got), len(groups))
	}
	for _, g := range groups {
		if len(g) != 4 || strings.Trim(g, alphabet) != "" {
			t.Errorf("group %q is not four characters of the alphabet", g)
		}
	}
	compact := Compact(got)
	if len(compact) != 52 || strings.Contains(compact, "-") {
		t.Errorf("Compact gives %d characters, want 52 without separators", len(compact))
	}
	sum := sha256.Sum256(append(append([]byte(Label), sequence(0x40)...), sequence(0x60)...))
	if compact != Encode(sum[:]) {
		t.Error("Compact(display) differs from the encoded hash")
	}
}

func TestKeyOrderMatters(t *testing.T) {
	a, errA := Compute(sequence(0x00), sequence(0x20))
	b, errB := Compute(sequence(0x20), sequence(0x00))
	if errA != nil || errB != nil {
		t.Fatal(errA, errB)
	}
	if a == b {
		t.Error("swapping the keys gives the same fingerprint")
	}
}

// fileVector is one entry of testdata/noise/fingerprint.json. The field names avoid the
// keywords of the gitleaks rule generic-api-key.
type fileVector struct {
	NoisePublic   string `json:"noise_public"`
	Ed25519Public string `json:"ed25519_public"`
	Fingerprint   string `json:"fingerprint"`
}

type vectorFile struct {
	Format  int          `json:"format"`
	Vectors []fileVector `json:"vectors"`
}

const vectorFilePath = "../../testdata/noise/fingerprint.json"

// generateVectorFile builds fingerprint.json: the known vector first, then eight entries whose
// keys are SHA-256 of "fingerprint/noise/<i>" and "fingerprint/auth/<i>".
func generateVectorFile(t *testing.T) []byte {
	t.Helper()
	type pair struct{ noise, auth []byte }
	pairs := []pair{{sequence(0x00), sequence(0x20)}}
	for i := 1; i <= 8; i++ {
		n := sha256.Sum256([]byte("fingerprint/noise/" + itoa(i)))
		a := sha256.Sum256([]byte("fingerprint/auth/" + itoa(i)))
		pairs = append(pairs, pair{n[:], a[:]})
	}
	file := vectorFile{Format: 1}
	for _, p := range pairs {
		fp, err := Compute(p.noise, p.auth)
		if err != nil {
			t.Fatal(err)
		}
		file.Vectors = append(file.Vectors, fileVector{hex.EncodeToString(p.noise), hex.EncodeToString(p.auth), fp})
	}
	out, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(out, '\n')
}

func itoa(i int) string { return strconv.Itoa(i) }

func TestFingerprintFileCurrent(t *testing.T) {
	want := generateVectorFile(t)
	path := filepath.FromSlash(vectorFilePath)
	if os.Getenv("UPDATE_RECORDINGS") == "1" {
		if err := os.WriteFile(path, want, 0o644); err != nil { //nolint:gosec // G306: test data in the repository, read by everyone
			t.Fatal(err)
		}
		return
	}
	got, err := os.ReadFile(path) //nolint:gosec // G304: a fixed path of the repository's test data
	if err != nil {
		t.Fatalf("%v; run make -C services/host recordings", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s is not current (%d bytes, fresh generation %d bytes); run make -C services/host recordings", vectorFilePath, len(got), len(want))
	}
	var file vectorFile
	if err := json.Unmarshal(got, &file); err != nil || file.Format != 1 || len(file.Vectors) != 9 {
		t.Fatalf("fingerprint.json: format %d, %d vectors, err %v", file.Format, len(file.Vectors), err)
	}
}
