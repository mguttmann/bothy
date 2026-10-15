// SPDX-License-Identifier: Apache-2.0

package apitoken_test

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/identity/domain/apitoken"
)

// kinds are the four token kinds of 08 section 12.
var kinds = []apitoken.Kind{apitoken.KindAgent, apitoken.KindApp, apitoken.KindPersonal, apitoken.KindIncomingWebhook}

// TestFormat_RoundTrip generates a token of every kind and parses it back: length 60, the prefix,
// the kind at its position, the random part and the checksum (ADR 0061 "Confirmation").
func TestFormat_RoundTrip(t *testing.T) {
	t.Parallel()
	for _, k := range kinds {
		tok, err := apitoken.Generate(nil, k)
		if err != nil {
			t.Fatalf("Generate(%s): %v", k, err)
		}
		if len(tok) != apitoken.Length || apitoken.Length != 60 {
			t.Fatalf("len(%q) = %d, want 60", tok, len(tok))
		}
		if !strings.HasPrefix(tok, "bothy_"+string(k)+"_") || tok[53] != '_' {
			t.Fatalf("token %q: prefix or checksum separator out of place", tok)
		}
		p, err := apitoken.Parse(tok)
		if err != nil {
			t.Fatalf("Parse(%q): %v", tok, err)
		}
		if p.Kind != k || p.Random != tok[10:53] || p.Checksum != tok[54:] {
			t.Fatalf("Parse(%q) = %+v", tok, p)
		}
		if p.Checksum != apitoken.Checksum(p.Random) {
			t.Fatalf("checksum %q, want %q", p.Checksum, apitoken.Checksum(p.Random))
		}
		if !apitoken.HasPrefix(tok) {
			t.Fatalf("HasPrefix(%q) = false", tok)
		}
	}
	if apitoken.HasPrefix(strings.ToUpper(apitoken.Prefix[:1])+apitoken.Prefix[1:]+"agt_x") || apitoken.HasPrefix("eyJhbGciOi") {
		t.Fatal("HasPrefix accepts a value without the product prefix")
	}
}

// vector is one entry of testdata/checksum_vectors.json, shared with
// tools/ci/tests/test_gitleaks_rules.py.
type vector struct {
	Random   string `json:"random"`
	Checksum string `json:"checksum"`
}

// readVectors reads the checksum vectors.
func readVectors(t *testing.T) []vector {
	t.Helper()
	raw, err := os.ReadFile("testdata/checksum_vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Vectors []vector `json:"vectors"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Vectors) < 3 {
		t.Fatalf("only %d vectors", len(doc.Vectors))
	}
	return doc.Vectors
}

// TestFormat_ChecksumVectors checks every vector of testdata/checksum_vectors.json: random parts
// with the marker EXAMPLE (CONTRIBUTING.md), at least one checksum padded with a leading 0.
func TestFormat_ChecksumVectors(t *testing.T) {
	t.Parallel()
	padded := false
	for _, v := range readVectors(t) {
		if !strings.HasPrefix(v.Random, "EXAMPLE") || len(v.Random) != apitoken.RandomLen {
			t.Fatalf("vector random %q must start with EXAMPLE and have 43 characters", v.Random)
		}
		if got := apitoken.Checksum(v.Random); got != v.Checksum {
			t.Errorf("Checksum(%q) = %q, want %q", v.Random, got, v.Checksum)
		}
		padded = padded || strings.HasPrefix(v.Checksum, "0")
		tok := "bothy_agt_" + v.Random + "_" + v.Checksum
		if _, err := apitoken.Parse(tok); err != nil {
			t.Errorf("Parse(vector %q): %v", v.Random, err)
		}
	}
	if !padded {
		t.Fatal("no vector has a checksum with a leading 0")
	}
}

// TestParse_Rejects lists the malformed values of the brief; each is refused without a lookup.
func TestParse_Rejects(t *testing.T) {
	t.Parallel()
	v := readVectors(t)[0]
	good := "bothy_agt_" + v.Random + "_" + v.Checksum
	if _, err := apitoken.Parse(good); err != nil {
		t.Fatalf("Parse(good) = %v", err)
	}
	flip := func(s string, i int) string {
		b := []byte(s)
		if b[i] == 'A' {
			b[i] = 'B'
		} else {
			b[i] = 'A'
		}
		return string(b)
	}
	high := strings.Repeat("z", 43)
	cases := map[string]string{
		"empty":               "",
		"wrong prefix":        "bothx" + good[5:],
		"upper case prefix":   strings.ToUpper(good[:5]) + good[5:],
		"unknown kind":        "bothy_abc_" + good[10:],
		"kind hpc":            "bothy_hpc_" + good[10:],
		"random 42":           "bothy_agt_" + v.Random[:42] + "_" + apitoken.Checksum(v.Random[:42]),
		"random 44":           "bothy_agt_" + v.Random + "A_" + apitoken.Checksum(v.Random+"A"),
		"non-Base62":          "bothy_agt_" + v.Random[:42] + "-_" + apitoken.Checksum(v.Random[:42]+"-"),
		"changed checksum":    flip(good, len(good)-1),
		"changed random":      flip(good, 20),
		"checksum 5":          good[:len(good)-1],
		"checksum 7":          good + "0",
		"random at max":       "bothy_agt_" + high + "_" + apitoken.Checksum(high),
		"random equals 2^256": "bothy_agt_" + twoTo256 + "_" + apitoken.Checksum(twoTo256),
		"trailing newline":    good + "\n",
		"leading space":       " " + good,
	}
	for name, s := range cases {
		if _, err := apitoken.Parse(s); !errors.Is(err, apitoken.ErrMalformed) {
			t.Errorf("%s: Parse = %v, want ErrMalformed", name, err)
		}
	}
	below := twoTo256Minus1
	if _, err := apitoken.Parse("bothy_agt_" + below + "_" + apitoken.Checksum(below)); err != nil {
		t.Fatalf("2^256-1 is a valid random part: %v", err)
	}
}

// twoTo256 is 2^256 in Base62 with 43 digits; twoTo256Minus1 the largest random part.
const (
	twoTo256       = "yhjskwdA6OZ1AL1YmHWZWm8LLG7HjnuCA2j5rOw8Xp2"
	twoTo256Minus1 = "yhjskwdA6OZ1AL1YmHWZWm8LLG7HjnuCA2j5rOw8Xp1"
)

// countingReader records how many bytes were read from it.
type countingReader struct {
	r io.Reader
	n int
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += n
	return n, err
}

// TestGenerate_ReadsExactly32BytesFromCryptoRand is ID-T27 for API tokens: the random part is
// exactly 256 bits read from the injected source (crypto/rand.Reader in production), written as
// 43 Base62 digits; a short source is an error, never a weaker token.
func TestGenerate_ReadsExactly32BytesFromCryptoRand(t *testing.T) {
	t.Parallel()
	src := &countingReader{r: bytes.NewReader(bytes.Repeat([]byte{0xff}, 64))}
	tok, err := apitoken.Generate(src, apitoken.KindAgent)
	if err != nil {
		t.Fatal(err)
	}
	if src.n != 32 {
		t.Fatalf("read %d bytes, want 32", src.n)
	}
	p, err := apitoken.Parse(tok)
	if err != nil {
		t.Fatal(err)
	}
	if p.Random != twoTo256Minus1 {
		t.Fatalf("random of 32 bytes 0xff = %q, want %q", p.Random, twoTo256Minus1)
	}
	zero, err := apitoken.Generate(bytes.NewReader(make([]byte, 32)), apitoken.KindApp)
	if err != nil {
		t.Fatal(err)
	}
	if want := strings.Repeat("0", 43); zero[10:53] != want {
		t.Fatalf("zero random = %q, want left-padded zeros", zero[10:53])
	}
	if _, err := apitoken.Generate(bytes.NewReader(make([]byte, 31)), apitoken.KindAgent); err == nil {
		t.Fatal("a source with 31 bytes must fail")
	}
	if _, err := apitoken.Generate(nil, apitoken.Kind("hpc")); err == nil {
		t.Fatal("an unknown kind must fail")
	}
	a, _ := apitoken.Generate(nil, apitoken.KindAgent)
	b, _ := apitoken.Generate(nil, apitoken.KindAgent)
	if a == b {
		t.Fatal("two tokens from crypto/rand are equal")
	}
}

// TestTokenPrefix checks token_prefix (bothy_<kind>_ plus 6 characters of the random part) and
// the HMAC (03 api_tokens).
func TestTokenPrefix(t *testing.T) {
	t.Parallel()
	v := readVectors(t)[0]
	tok := "bothy_pat_" + v.Random + "_" + v.Checksum
	p, err := apitoken.Parse(tok)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := p.Prefix(), "bothy_pat_"+v.Random[:6]; got != want || len(got) != 16 {
		t.Fatalf("Prefix() = %q, want %q", got, want)
	}
	pepper := []byte("0123456789abcdef0123456789abcdef")
	m := hmac.New(sha256.New, pepper)
	m.Write([]byte(tok))
	if !hmac.Equal(apitoken.Hash(pepper, tok), m.Sum(nil)) {
		t.Fatal("Hash is not HMAC-SHA-256 of the full token with the pepper")
	}
}
