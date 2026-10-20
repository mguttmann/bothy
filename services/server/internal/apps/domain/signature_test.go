// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	standardwebhooks "github.com/standard-webhooks/standard-webhooks/libraries/go"

	"github.com/OWNER/bothy/services/server/internal/apps/domain"
	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// vectorFile is the contract file of the signatures (docs/api/webhook-test-vectors.json).
type vectorFile struct {
	Version    int                `json:"version"`
	Derivation []derivationVector `json:"derivation"`
	Signatures []signatureVector  `json:"signatures"`
}

type derivationVector struct {
	MasterHex      string `json:"master_hex"`
	SubscriptionID string `json:"subscription_id"`
	Generation     int    `json:"generation"`
	DerivedHex     string `json:"derived_hex"`
}

type signatureVector struct {
	Name              string   `json:"name"`
	WebhookID         string   `json:"webhook_id"`
	Timestamp         int64    `json:"timestamp"`
	Body              string   `json:"body"`
	Ed25519SeedBase64 []string `json:"ed25519_seed_base64"`
	PublicKey         []string `json:"public_key"`
	HMACSecrets       []string `json:"hmac_secrets"`
	SignatureHeader   string   `json:"signature_header"`
}

// vectorPath is the vector file in the repository.
func vectorPath(t *testing.T) string {
	return filepath.Join(repoRoot(t), "docs", "api", "webhook-test-vectors.json")
}

// signCase computes the header of v from its seeds and secrets with the domain functions.
func signCase(t *testing.T, v signatureVector) string {
	t.Helper()
	content := domain.SignedContent(ids.MustParse(v.WebhookID), v.Timestamp, []byte(v.Body))
	var v1a, v1 [][]byte
	for _, s := range v.Ed25519SeedBase64 {
		seed, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			t.Fatal(err)
		}
		v1a = append(v1a, ed25519.Sign(ed25519.NewKeyFromSeed(seed), content))
	}
	for _, s := range v.HMACSecrets {
		key, err := domain.ParseSecret(s)
		if err != nil {
			t.Fatal(err)
		}
		v1 = append(v1, domain.HMACSignature(key, content))
	}
	return domain.SignatureHeader(v1a, v1)
}

// writeVectors regenerates the vector file (go test ./internal/apps/domain -run
// TestWriteWebhookTestVectors -args -write-vectors).
var writeVectors = flag.Bool("write-vectors", false, "regenerate docs/api/webhook-test-vectors.json")

// TestWriteWebhookTestVectors writes the vector file from fixed seeds with -write-vectors (once, in
// M2-26); otherwise it is skipped and TestWebhookTestVectors checks the committed file.
func TestWriteWebhookTestVectors(t *testing.T) {
	if !*writeVectors {
		t.Skip("run with -args -write-vectors to regenerate docs/api/webhook-test-vectors.json")
	}
	seed := func(b byte) string {
		return base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{b}, ed25519.SeedSize))
	}
	pub := func(s string) string {
		raw, _ := base64.StdEncoding.DecodeString(s)
		k := ed25519.NewKeyFromSeed(raw).Public().(ed25519.PublicKey)
		return instance.FormatWebhookPublicKey(k)
	}
	master := make([]byte, 32)
	for i := range master {
		master[i] = byte(i)
	}
	sub1, sub2 := "0192f4a0-7b1c-7e12-8162-c8db90be34b1", "0192f4a0-7b1c-7e12-8162-c8db90be34b2"
	var f vectorFile
	f.Version = 1
	for _, d := range []struct {
		sub string
		gen int
	}{{sub1, 0}, {sub1, 1}, {sub2, 0}} {
		key := domain.DeriveHMACKey(master, ids.MustParse(d.sub), d.gen)
		f.Derivation = append(f.Derivation, derivationVector{MasterHex: hex.EncodeToString(master), SubscriptionID: d.sub,
			Generation: d.gen, DerivedHex: hex.EncodeToString(key)})
	}
	s1, s2 := "whsec_EXAMPLEvect1", "whsec_EXAMPLEvect2"
	id := "0192f4a0-7b1c-757c-87c8-267db4b2c89f"
	cases := []signatureVector{
		{Name: "one key and one secret", Body: `{"id":"` + id + `","type":"message.created"}`,
			Ed25519SeedBase64: []string{seed(1)}, HMACSecrets: []string{s1}},
		{Name: "two v1a signers", Body: `{"id":"` + id + `","type":"instance.updated"}`, Ed25519SeedBase64: []string{seed(2), seed(3)}},
		{Name: "two hmac generations", Body: `{"id":"` + id + `","type":"thread.created"}`,
			Ed25519SeedBase64: []string{seed(4)}, HMACSecrets: []string{s2, s1}},
		{Name: "non-ascii and escapes", Body: `{"payload":{"content_md":"Café \"Alex\" \\ Robin 😀 naïve 日本\n"}}`,
			Ed25519SeedBase64: []string{seed(5)}, HMACSecrets: []string{s1}},
		{Name: "truncated payload", Body: `{"id":"` + id + `","truncated":true,"payload":{}}`,
			Ed25519SeedBase64: []string{seed(6)}, HMACSecrets: []string{s2}},
	}
	for i := range cases {
		c := &cases[i]
		c.WebhookID, c.Timestamp = id, 1760346000+int64(i)
		for _, s := range c.Ed25519SeedBase64 {
			c.PublicKey = append(c.PublicKey, pub(s))
		}
		if c.HMACSecrets == nil {
			c.HMACSecrets = []string{}
		}
		c.SignatureHeader = signCase(t, *c)
	}
	f.Signatures = cases
	out, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(vectorPath(t), append(out, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestWebhookTestVectors is G10 part 1 (06 section 15, ADR 0016): the domain functions reproduce
// every committed header byte for byte; every v1a signature verifies with crypto/ed25519 against
// the published key form, every v1 signature with the Standard Webhooks reference library; a body
// changed by one byte fails both; every derivation reproduces derived_hex.
func TestWebhookTestVectors(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(vectorPath(t))
	if err != nil {
		t.Fatal(err)
	}
	var f vectorFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	if f.Version != 1 || len(f.Signatures) < 5 || len(f.Derivation) < 2 {
		t.Fatalf("vector file incomplete: %+v", f)
	}
	for _, d := range f.Derivation {
		master, err := hex.DecodeString(d.MasterHex)
		if err != nil {
			t.Fatal(err)
		}
		key := domain.DeriveHMACKey(master, ids.MustParse(d.SubscriptionID), d.Generation)
		if hex.EncodeToString(key) != d.DerivedHex {
			t.Errorf("derivation %s/%d differs", d.SubscriptionID, d.Generation)
		}
		back, err := domain.ParseSecret(domain.FormatSecret(key))
		if err != nil || !bytes.Equal(back, key) {
			t.Errorf("FormatSecret and ParseSecret do not round-trip: %v", err)
		}
	}
	for _, v := range f.Signatures {
		t.Run(v.Name, func(t *testing.T) {
			t.Parallel()
			for _, s := range v.HMACSecrets {
				if !strings.Contains(s, "EXAMPLE") {
					t.Fatalf("secret without the EXAMPLE marker")
				}
			}
			if got := signCase(t, v); got != v.SignatureHeader {
				t.Fatalf("header\n got %s\nwant %s", got, v.SignatureHeader)
			}
			checkVector(t, v, []byte(v.Body), true)
			changed := []byte(v.Body)
			changed[len(changed)/2] ^= 0x01
			checkVector(t, v, changed, false)
		})
	}
}

// checkVector verifies the header of v over body with the standard library (v1a) and the
// reference library (v1); want says whether they must verify.
func checkVector(t *testing.T, v signatureVector, body []byte, want bool) {
	t.Helper()
	content := domain.SignedContent(ids.MustParse(v.WebhookID), v.Timestamp, body)
	var v1a []string
	for _, e := range strings.Split(v.SignatureHeader, " ") {
		if s, ok := strings.CutPrefix(e, "v1a,"); ok {
			v1a = append(v1a, s)
		}
	}
	if len(v1a) != len(v.PublicKey) {
		t.Fatalf("%d v1a entries for %d keys", len(v1a), len(v.PublicKey))
	}
	for i, s := range v1a {
		der, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(v.PublicKey[i], "whpk_"))
		if err != nil {
			t.Fatal(err)
		}
		k, err := x509.ParsePKIXPublicKey(der)
		if err != nil {
			t.Fatal(err)
		}
		sig, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			t.Fatal(err)
		}
		if ed25519.Verify(k.(ed25519.PublicKey), content, sig) != want {
			t.Fatalf("v1a %d verifies %v, want %v", i, !want, want)
		}
	}
	h := http.Header{}
	h.Set("webhook-id", v.WebhookID)
	h.Set("webhook-timestamp", strconv.FormatInt(v.Timestamp, 10))
	h.Set("webhook-signature", v.SignatureHeader)
	for _, s := range v.HMACSecrets {
		wh, err := standardwebhooks.NewWebhook(s)
		if err != nil {
			t.Fatal(err)
		}
		if err := wh.VerifyIgnoringTimestamp(body, h); (err == nil) != want {
			t.Fatalf("v1 with a secret: %v, want verified %v", err, want)
		}
	}
}

func TestSignatureHeaderFormat(t *testing.T) {
	t.Parallel()
	got := domain.SignatureHeader([][]byte{{1, 2, 3}, {4}}, [][]byte{{5, 6}})
	if got != "v1a,AQID v1a,BA== v1,BQY=" {
		t.Fatalf("header %q", got)
	}
	if got := domain.SignatureHeader([][]byte{{1}}, nil); got != "v1a,AQ==" {
		t.Fatalf("header without hmac %q", got)
	}
	id := ids.MustParse("0192f4a0-7b1c-757c-87c8-267db4b2c89f")
	if c := string(domain.SignedContent(id, 1760346000, []byte(`{"a":1}`))); c != `0192f4a0-7b1c-757c-87c8-267db4b2c89f.1760346000.{"a":1}` {
		t.Fatalf("signed content %q", c)
	}
	if _, err := domain.ParseSecret("whsk_AAAA"); err == nil {
		t.Fatal("foreign prefix accepted")
	}
	if _, err := domain.ParseSecret("whsec_!!!"); err == nil {
		t.Fatal("bad base64 accepted")
	}
	if s := domain.FormatSecret([]byte{0xff, 0xfe}); s != "whsec_//4=" {
		t.Fatalf("secret form %q", s)
	}
}

func TestHMACOverlap(t *testing.T) {
	t.Parallel()
	rotated := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	overlap := 24 * time.Hour
	cases := []struct {
		gen     int
		rotated time.Time
		now     time.Time
		want    []int
	}{
		{0, time.Time{}, rotated, []int{0}},
		{1, rotated, rotated.Add(time.Hour), []int{1, 0}},
		{3, rotated, rotated.Add(overlap - time.Nanosecond), []int{3, 2}},
		{3, rotated, rotated.Add(overlap), []int{3}},
		{3, rotated, rotated.Add(48 * time.Hour), []int{3}},
		{2, time.Time{}, rotated, []int{2}},
	}
	for _, c := range cases {
		if got := domain.HMACGenerations(c.gen, c.rotated, overlap, c.now); !slices.Equal(got, c.want) {
			t.Errorf("HMACGenerations(%d, %v, %v) = %v, want %v", c.gen, c.rotated, c.now, got, c.want)
		}
	}
}

func TestDeriveHMACKeyStable(t *testing.T) {
	t.Parallel()
	master := make([]byte, 32)
	for i := range master {
		master[i] = byte(i)
	}
	a := ids.MustParse("0192f4a0-7b1c-7e12-8162-c8db90be34b1")
	b := ids.MustParse("0192f4a0-7b1c-7e12-8162-c8db90be34b2")
	k := domain.DeriveHMACKey(master, a, 0)
	if len(k) != 32 || !bytes.Equal(k, domain.DeriveHMACKey(master, a, 0)) {
		t.Fatal("derivation not stable")
	}
	if bytes.Equal(k, domain.DeriveHMACKey(master, a, 1)) || bytes.Equal(k, domain.DeriveHMACKey(master, b, 0)) ||
		bytes.Equal(k, domain.DeriveHMACKey(append([]byte{1}, master[1:]...), a, 0)) {
		t.Fatal("another subscription, generation or master gives the same key")
	}
}
