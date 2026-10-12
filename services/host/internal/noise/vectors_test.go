// SPDX-License-Identifier: Apache-2.0

package noise

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const testdataDir = "../../testdata/noise"

// vectorFiles are the public test vectors of TM-T01 (docs/spec/24-terminal.md 5.2, D-T19).
var vectorFiles = []string{"cacophony.txt.gz", "noise-c-basic.txt.gz"}

// vectorMessage is one message of a vector entry: handshake messages first, then transport.
type vectorMessage struct {
	Payload    hexBytes `json:"payload"`
	Ciphertext hexBytes `json:"ciphertext"`
}

// vectorEntry holds the fields of the vector file format (Noise wiki "Test vectors") that this
// test reads; unknown fields are ignored.
type vectorEntry struct {
	ProtocolName     string          `json:"protocol_name"`
	Name             string          `json:"name"`
	Pattern          string          `json:"pattern"`
	DH               string          `json:"dh"`
	Cipher           string          `json:"cipher"`
	Hash             string          `json:"hash"`
	Fail             bool            `json:"fail"`
	Fallback         bool            `json:"fallback"`
	InitPrologue     hexBytes        `json:"init_prologue"`
	RespPrologue     *hexBytes       `json:"resp_prologue"`
	InitEphemeral    hexBytes        `json:"init_ephemeral"`
	InitRemoteStatic hexBytes        `json:"init_remote_static"`
	RespStatic       hexBytes        `json:"resp_static"`
	RespEphemeral    hexBytes        `json:"resp_ephemeral"`
	InitPSK          hexBytes        `json:"init_psk"`
	RespPSK          hexBytes        `json:"resp_psk"`
	InitPSKs         []hexBytes      `json:"init_psks"`
	RespPSKs         []hexBytes      `json:"resp_psks"`
	HandshakeHash    hexBytes        `json:"handshake_hash"`
	Messages         []vectorMessage `json:"messages"`
}

type hexBytes []byte

func (h *hexBytes) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	v, err := hex.DecodeString(s)
	*h = v
	return err
}

// protocol returns the protocol of an entry by the loader rules of the brief (Design note 10).
func (e vectorEntry) protocol() string {
	switch {
	case e.ProtocolName != "":
		return e.ProtocolName
	case e.Name != "":
		return e.Name
	default:
		return fmt.Sprintf("Noise_%s_%s_%s_%s", e.Pattern, e.DH, e.Cipher, e.Hash)
	}
}

func (e vectorEntry) selected() bool {
	return e.protocol() == ProtocolName && len(e.InitPSK) == 0 && len(e.RespPSK) == 0 &&
		len(e.InitPSKs) == 0 && len(e.RespPSKs) == 0 && !e.Fallback
}

type indexedEntry struct {
	index int
	entry vectorEntry
}

func readGzip(t *testing.T, name string) []byte {
	t.Helper()
	f, err := os.Open(filepath.Join(testdataDir, "vectors", name)) //nolint:gosec // G304: a fixed file of the repository's test data
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// loadVectors returns the selected entries of one file and the names of all entries.
func loadVectors(t *testing.T, name string) (selected []indexedEntry, protocols []string) {
	t.Helper()
	var file struct {
		Vectors []vectorEntry `json:"vectors"`
	}
	if err := json.Unmarshal(readGzip(t, name), &file); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	for i, e := range file.Vectors {
		protocols = append(protocols, e.protocol())
		if e.selected() {
			selected = append(selected, indexedEntry{i, e})
		}
	}
	return selected, protocols
}

func TestVectorsTMT01(t *testing.T) {
	for _, name := range vectorFiles {
		selected, protocols := loadVectors(t, name)
		if len(selected) == 0 {
			t.Fatalf("%s yields no %s entry", name, ProtocolName)
		}
		for _, p := range protocols {
			if p == "NoisePSK_NK_25519_ChaChaPoly_SHA256" {
				for _, s := range selected {
					if s.entry.protocol() == p {
						t.Errorf("%s: the PSK entry of an older revision is selected", name)
					}
				}
			}
		}
		for _, s := range selected {
			t.Run(fmt.Sprintf("%s/%d", strings.TrimSuffix(name, ".txt.gz"), s.index), func(t *testing.T) {
				err := replayVector(t, s.entry)
				if s.entry.Fail && err == nil {
					t.Fatal("an entry with fail: true succeeded")
				}
				if !s.entry.Fail && err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

// replayVector runs one entry and returns the first mismatch or failure.
func replayVector(t *testing.T, e vectorEntry) error {
	t.Helper()
	if len(e.Messages) < 2 {
		return fmt.Errorf("entry has %d messages, want at least 2", len(e.Messages))
	}
	initEph, err := KeyPairFromPrivate([KeyLen]byte(e.InitEphemeral))
	if err != nil {
		return err
	}
	respStatic, err := KeyPairFromPrivate([KeyLen]byte(e.RespStatic))
	if err != nil {
		return err
	}
	respEph, err := KeyPairFromPrivate([KeyLen]byte(e.RespEphemeral))
	if err != nil {
		return err
	}
	respPrologue := e.InitPrologue
	if e.RespPrologue != nil {
		respPrologue = *e.RespPrologue
	}
	init, err := newInitiator([KeyLen]byte(e.InitRemoteStatic), e.InitPrologue, initEph)
	if err != nil {
		return err
	}
	resp, err := newResponder(respStatic, respPrologue, &respEph, nil)
	if err != nil {
		return err
	}
	m1, err := init.WriteMessage1(e.Messages[0].Payload)
	if err != nil {
		return fmt.Errorf("message 1: %w", err)
	}
	if !bytes.Equal(m1, e.Messages[0].Ciphertext) {
		return fmt.Errorf("message 1 differs (%d bytes, want %d)", len(m1), len(e.Messages[0].Ciphertext))
	}
	p1, _, err := resp.ReadMessage1(m1)
	if err != nil {
		return fmt.Errorf("read message 1: %w", err)
	}
	if !bytes.Equal(p1, e.Messages[0].Payload) {
		return fmt.Errorf("payload 1 differs")
	}
	m2, respT, err := resp.WriteMessage2(e.Messages[1].Payload)
	if err != nil {
		return fmt.Errorf("message 2: %w", err)
	}
	if !bytes.Equal(m2, e.Messages[1].Ciphertext) {
		return fmt.Errorf("message 2 differs")
	}
	p2, initT, err := init.ReadMessage2(m2)
	if err != nil {
		return fmt.Errorf("read message 2: %w", err)
	}
	if !bytes.Equal(p2, e.Messages[1].Payload) {
		return fmt.Errorf("payload 2 differs")
	}
	if len(e.HandshakeHash) > 0 && (!bytes.Equal(init.HandshakeHash(), e.HandshakeHash) || !bytes.Equal(resp.HandshakeHash(), e.HandshakeHash)) {
		return fmt.Errorf("handshake hash differs")
	}
	for i, m := range e.Messages[2:] {
		sender, receiver := initT, respT
		if i%2 == 1 {
			sender, receiver = respT, initT
		}
		ct, err := sender.Encrypt(m.Payload)
		if err != nil {
			return fmt.Errorf("transport %d: %w", i, err)
		}
		if !bytes.Equal(ct, m.Ciphertext) {
			return fmt.Errorf("transport message %d differs", i)
		}
		pt, err := receiver.Decrypt(ct)
		if err != nil {
			return fmt.Errorf("transport message %d does not decrypt: %w", i, err)
		}
		if !bytes.Equal(pt, m.Payload) {
			return fmt.Errorf("transport payload %d differs", i)
		}
	}
	return nil
}

var provenanceRow = regexp.MustCompile("^\\| `vectors/([a-z0-9.-]+\\.txt\\.gz)` \\| (.+?) \\| `([0-9a-f]{40})` \\| `([A-Za-z0-9.-]+)` \\| `([0-9a-f]{64})` \\|$")

// TestVectorProvenance holds the README table of the vector files: every row names an existing
// file, a full commit, an SPDX license with its license file and the SHA-256 of the content.
func TestVectorProvenance(t *testing.T) {
	f, err := os.Open(filepath.Join(testdataDir, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	licenseFiles := map[string]string{"cacophony.txt.gz": "LICENSE.cacophony", "noise-c-basic.txt.gz": "LICENSE.noise-c"}
	seen := map[string]bool{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		m := provenanceRow.FindStringSubmatch(sc.Text())
		if m == nil {
			continue
		}
		file, license, sum := m[1], m[4], m[5]
		seen[file] = true
		lf, ok := licenseFiles[file]
		if !ok {
			t.Errorf("README names an unknown vector file %s", file)
			continue
		}
		if _, err := os.Stat(filepath.Join(testdataDir, "vectors", lf)); err != nil {
			t.Errorf("%s: license file %s: %v", file, lf, err)
		}
		if !acceptedTestDataLicense(t, license) {
			t.Errorf("%s: license %s is not accepted for test data", file, license)
		}
		got := sha256.Sum256(readGzip(t, file))
		if hex.EncodeToString(got[:]) != sum {
			t.Errorf("%s: SHA-256 of the content differs from the README", file)
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	for _, name := range vectorFiles {
		if !seen[name] {
			t.Errorf("README has no provenance row for %s", name)
		}
	}
}

// acceptedTestDataLicense reports whether a license may cover checked-in test data: one of
// quality/licenses/allowed.txt or the public-domain dedication Unlicense (brief M2-38, note 10).
func acceptedTestDataLicense(t *testing.T, license string) bool {
	t.Helper()
	if license == "Unlicense" {
		return true
	}
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "quality", "licenses", "allowed.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(b), "\n") {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") && line == license {
			return true
		}
	}
	return false
}
