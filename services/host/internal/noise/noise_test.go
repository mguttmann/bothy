// SPDX-License-Identifier: Apache-2.0

package noise

import (
	"bytes"
	"errors"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// The IDs of the Go recording (brief M2-38, Design note 11).
const (
	goSessionID = "0192f4a0-7b1c-7b22-9d4f-5a6b7c8d9e0f"
	goHostID    = "0192f4a0-7b1c-7b22-9d4f-5a6b7c8d9e10"
)

func mustID(t testing.TB, s string) [IDLen]byte {
	t.Helper()
	id, err := ParseID(s)
	if err != nil {
		t.Fatalf("ParseID: %v", err)
	}
	return id
}

func TestConstants(t *testing.T) {
	if ProtocolName != "Noise_NK_25519_ChaChaPoly_SHA256" || PrologueLabel != "bothy-terminal/1" ||
		len(PrologueLabel) != 16 || MaxMessageLen != 65535 || TagLen != 16 ||
		MaxPlaintextLen != 65519 || KeyLen != 32 || IDLen != 16 {
		t.Error("a protocol constant differs from docs/spec/24-terminal.md 5.2")
	}
}

func TestPrologue(t *testing.T) {
	p := Prologue(mustID(t, goSessionID), mustID(t, goHostID))
	want := []byte("bothy-terminal/1")
	want = append(want, 0x01, 0x92, 0xf4, 0xa0, 0x7b, 0x1c, 0x7b, 0x22, 0x9d, 0x4f, 0x5a, 0x6b, 0x7c, 0x8d, 0x9e, 0x0f)
	want = append(want, 0x01, 0x92, 0xf4, 0xa0, 0x7b, 0x1c, 0x7b, 0x22, 0x9d, 0x4f, 0x5a, 0x6b, 0x7c, 0x8d, 0x9e, 0x10)
	if len(p) != 48 || !bytes.Equal(p, want) {
		t.Errorf("Prologue = % x", p)
	}
}

func TestParseID(t *testing.T) {
	lower, err := ParseID(goSessionID)
	if err != nil {
		t.Fatal(err)
	}
	upper, err := ParseID(strings.ToUpper(goSessionID))
	if err != nil || upper != lower {
		t.Errorf("upper case: %v, equal %v", err, upper == lower)
	}
	bad := []string{
		goSessionID[:35],
		goSessionID + "0",
		"{" + goSessionID + "}",
		"urn:uuid:" + goSessionID,
		"0192f4a0-7b1c-7b22-9d4f-5a6b7c8d9e0g",
		"0192f4a07-b1c-7b22-9d4f-5a6b7c8d9e0f",
		"0192f4a0+7b1c-7b22-9d4f-5a6b7c8d9e0f",
		"0192f4a0-7b1c-7b22-9d4f-5a6b7c8d9e0 ",
		"",
	}
	for _, s := range bad {
		if _, err := ParseID(s); !errors.Is(err, ErrInvalidID) {
			t.Errorf("ParseID of a %d-character string: err = %v, want ErrInvalidID", len(s), err)
		}
	}
}

func TestEphemeralKeyHash(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	got, err := EphemeralKeyHash(key)
	if err != nil || got != "Yw3NKWbEM2aRElRIu7JbT_QSpJxzLbLIq8G4WBvXEN0" {
		t.Errorf("EphemeralKeyHash = %q, %v", got, err)
	}
	for _, n := range []int{31, 33} {
		if _, err := EphemeralKeyHash(make([]byte, n)); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("%d bytes: err = %v, want ErrInvalidKey", n, err)
		}
	}
}

func TestKeyPairs(t *testing.T) {
	kp, err := GenerateKeyPair(bytes.NewReader(bytes.Repeat([]byte{7}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	again, err := KeyPairFromPrivate(kp.Private)
	if err != nil || again != kp {
		t.Errorf("KeyPairFromPrivate differs from GenerateKeyPair: %v", err)
	}
	if _, err := GenerateKeyPair(bytes.NewReader(make([]byte, 5))); err == nil {
		t.Error("a short random source gave a key pair")
	}
	if _, err := GenerateKeyPair(nil); err != nil {
		t.Errorf("nil random source (crypto/rand): %v", err)
	}
}

// TestPackagesImportNoNetwork holds the brief's rule that the Noise, frame and fingerprint
// packages do no networking and start no process (R-TM-01, the network belongs to M2-43/M2-44).
func TestPackagesImportNoNetwork(t *testing.T) {
	for _, dir := range []string{".", "../frame", "../fingerprint"} {
		files, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil || len(files) == 0 {
			t.Fatalf("%s: no Go files (%v)", dir, err)
		}
		for _, file := range files {
			if strings.HasSuffix(file, "_test.go") {
				continue
			}
			f, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ImportsOnly)
			if err != nil {
				t.Fatal(err)
			}
			for _, imp := range f.Imports {
				path, _ := strconv.Unquote(imp.Path.Value)
				if path == "net" || strings.HasPrefix(path, "net/") || path == "os/exec" || path == "log" || path == "log/slog" {
					t.Errorf("%s imports %s", file, path)
				}
			}
		}
	}
	if _, err := os.Stat("../frame/frame.go"); err != nil {
		t.Fatal(err)
	}
}
