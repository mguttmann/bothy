// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/files/domain"
)

func TestPlanParts(t *testing.T) {
	for _, c := range []struct {
		name            string
		size, partSize  int64
		wantCount       int
		wantLast        int64
		wantLastPartLen int64
	}{
		{"equal to the part size", 8, 8, 1, 8, 8},
		{"one byte more", 9, 8, 2, 1, 1},
		{"smaller", 3, 8, 1, 3, 3},
		{"exact multiple", 24, 8, 3, 8, 8},
		{"rounded up", 25, 8, 4, 1, 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			count, last := domain.PlanParts(c.size, c.partSize)
			if count != c.wantCount || last != c.wantLast {
				t.Fatalf("PlanParts(%d, %d) = %d, %d; want %d, %d", c.size, c.partSize, count, last, c.wantCount, c.wantLast)
			}
			var sum int64
			for n := 1; n <= count; n++ {
				l := domain.PartLength(n, count, c.size, c.partSize)
				if n < count && l != c.partSize {
					t.Errorf("PartLength(%d) = %d, want the part size %d", n, l, c.partSize)
				}
				sum += l
			}
			if got := domain.PartLength(count, count, c.size, c.partSize); got != c.wantLastPartLen {
				t.Errorf("last part length = %d, want %d", got, c.wantLastPartLen)
			}
			if sum != c.size {
				t.Errorf("sum of the part lengths = %d, want %d", sum, c.size)
			}
		})
	}
}

func TestPartLength(t *testing.T) {
	count, _ := domain.PlanParts(20, 8)
	for _, c := range []struct {
		n    int
		want int64
	}{{1, 8}, {2, 8}, {3, 4}, {0, 0}, {4, 0}, {-1, 0}} {
		if got := domain.PartLength(c.n, count, 20, 8); got != c.want {
			t.Errorf("PartLength(%d) = %d, want %d", c.n, got, c.want)
		}
	}
}

// nameLimit is the file.name_max_bytes of the tests.
const nameLimit = 32

func TestCleanName(t *testing.T) {
	for _, c := range []struct {
		name, in, want string
		limit          int
	}{
		{"plain", "report.pdf", "report.pdf", nameLimit},
		{"right-to-left override", "invoice\u202Efdp.exe", "invoicefdp.exe", nameLimit},
		{"parent path", "../../x.pdf", "x.pdf", nameLimit},
		{"backslash path", `a\b.txt`, "b.txt", nameLimit},
		{"control characters", "a\x00b\tc\x7fd\u0085.txt", "abcd.txt", nameLimit},
		{"zero width", "a\u200Bb\u200Cc\u200Dd\u2060e\uFEFF.txt", "abcde.txt", nameLimit},
		{"bidi marks", "\u061Ca\u200Eb\u200Fc\u2066d\u2069.txt", "abcd.txt", nameLimit},
		{"trimmed", "  spaced name.txt \u3000", "spaced name.txt", nameLimit},
		{"dot", ".", "file", nameLimit},
		{"dot dot", "..", "file", nameLimit},
		{"empty", "", "file", nameLimit},
		{"blank", " \t ", "file", nameLimit},
		{"only a path", "dir/", "file", nameLimit},
		{"only controls", "\u202E\u200B", "file", nameLimit},
		{"invalid UTF-8 dropped", "a\xffb.txt", "ab.txt", nameLimit},
		{"keeps the extension", strings.Repeat("a", 40) + ".pdf", strings.Repeat("a", nameLimit-4) + ".pdf", nameLimit},
		{"never splits a rune", strings.Repeat("ä", 20) + ".pdf", strings.Repeat("ä", (nameLimit-4)/2) + ".pdf", nameLimit},
		{"extension too long", "a." + strings.Repeat("b", 40), "a." + strings.Repeat("b", nameLimit-2), nameLimit},
		{"no extension", strings.Repeat("ü", 20), strings.Repeat("ü", nameLimit/2), nameLimit},
		{"cut leaves a trailing space", "abc " + strings.Repeat("d", 40), "abc", 4},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := domain.CleanName(c.in, c.limit)
			if got != c.want {
				t.Fatalf("CleanName(%q) = %q, want %q", c.in, got, c.want)
			}
			if len(got) > c.limit {
				t.Fatalf("CleanName(%q) has %d bytes, limit %d", c.in, len(got), c.limit)
			}
		})
	}
}

func FuzzCleanName(f *testing.F) {
	for _, s := range []string{"report.pdf", "../../x.pdf", "a\\b.txt", "\u202Etxt.exe", ".", "..", "", " ",
		strings.Repeat("ä", 200) + ".tar.gz", "a\x00\u200Bb", "\xff\xfe"} {
		f.Add(s, 255)
		f.Add(s, 8)
	}
	f.Fuzz(func(t *testing.T, in string, limit int) {
		if limit < domain.MinNameBytes || limit > 4096 {
			return
		}
		out := domain.CleanName(in, limit)
		switch {
		case !utf8.ValidString(out):
			t.Fatalf("CleanName(%q) = %q is not valid UTF-8", in, out)
		case out == "":
			t.Fatalf("CleanName(%q) is empty", in)
		case len(out) > limit:
			t.Fatalf("CleanName(%q) = %q has %d bytes, limit %d", in, out, len(out), limit)
		case strings.ContainsAny(out, `/\`):
			t.Fatalf("CleanName(%q) = %q has a path separator", in, out)
		case out == "." || out == "..":
			t.Fatalf("CleanName(%q) = %q", in, out)
		}
		for _, r := range out {
			if unicode.Is(unicode.Cc, r) || domain.IsHiddenRune(r) {
				t.Fatalf("CleanName(%q) = %q keeps %U", in, out, r)
			}
		}
		if again := domain.CleanName(out, limit); again != out {
			t.Fatalf("CleanName is not stable: %q then %q", out, again)
		}
	})
}

// digestHeader is the Content-Digest of b with the member sha-256.
func digestHeader(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha-256=:" + base64.StdEncoding.EncodeToString(sum[:]) + ":"
}

func TestParseContentDigest(t *testing.T) {
	sum := sha256.Sum256([]byte("part"))
	b64 := base64.StdEncoding.EncodeToString(sum[:])
	for _, c := range []struct {
		name, in string
		ok       bool
	}{
		{"valid", "sha-256=:" + b64 + ":", true},
		{"other members ignored", `sha-512=:` + base64.StdEncoding.EncodeToString(make([]byte, 64)) + `:, sha-256=:` + b64 + `:;p=1, x="a,b"`, true},
		{"with white space", "  sha-256=:" + b64 + ":  ", true},
		{"without padding", "sha-256=:" + strings.TrimRight(b64, "=") + ":", true},
		{"missing member", "sha-512=:" + base64.StdEncoding.EncodeToString(make([]byte, 64)) + ":", false},
		{"empty", "", false},
		{"wrong length", "sha-256=:" + base64.StdEncoding.EncodeToString(sum[:31]) + ":", false},
		{"bad base64", "sha-256=:!!!!:", false},
		{"not a byte sequence", `sha-256="` + b64 + `"`, false},
		{"old Digest form", "SHA-256=" + b64, false},
		{"trailing comma", "sha-256=:" + b64 + ":,", false},
		{"unterminated", "sha-256=:" + b64, false},
		{"bare key", "sha-256", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, err := domain.ParseContentDigest(c.in)
			if c.ok {
				if err != nil || got != sum {
					t.Fatalf("ParseContentDigest(%q) = %x, %v; want %x", c.in, got, err, sum)
				}
				return
			}
			if err == nil {
				t.Fatalf("ParseContentDigest(%q) succeeded", c.in)
			}
			if !errors.Is(err, domain.ErrContentDigest) {
				t.Fatalf("error %v does not wrap ErrContentDigest", err)
			}
		})
	}
}

func FuzzParseContentDigest(f *testing.F) {
	f.Add(digestHeader([]byte("x")))
	f.Add(`sha-512=:AAAA:, sha-256=:` + base64.StdEncoding.EncodeToString(make([]byte, 32)) + `:`)
	f.Add(`a=("b" 1;x);y=?1, sha-256=:YQ==:`)
	f.Add(`sha-256=:`)
	f.Fuzz(func(t *testing.T, in string) {
		got, err := domain.ParseContentDigest(in)
		if err != nil {
			return
		}
		// An accepted header names exactly one SHA-256 digest as a byte sequence.
		if !strings.Contains(in, "sha-256=:") {
			t.Fatalf("ParseContentDigest(%q) = %x without a sha-256 byte sequence", in, got)
		}
	})
}

func TestSessionTransitions(t *testing.T) {
	for _, c := range []struct {
		from, to domain.SessionState
		ok       bool
	}{
		{domain.SessionOpen, domain.SessionCompleted, true},
		{domain.SessionOpen, domain.SessionAborted, true},
		{domain.SessionOpen, domain.SessionExpired, true},
		{domain.SessionCompleted, domain.SessionAborted, false},
		{domain.SessionAborted, domain.SessionCompleted, false},
		{domain.SessionExpired, domain.SessionOpen, false},
		{domain.SessionCompleted, domain.SessionCompleted, false},
		{domain.SessionState("unknown"), domain.SessionAborted, false},
	} {
		err := domain.CheckTransition(c.from, c.to)
		if (err == nil) != c.ok {
			t.Errorf("CheckTransition(%s, %s) = %v, want ok %v", c.from, c.to, err, c.ok)
		}
	}
}

func TestUploadStateView(t *testing.T) {
	for _, c := range []struct {
		state     domain.SessionState
		confirmed int
		want      string
		ok        bool
	}{
		{domain.SessionOpen, 0, "pending", true},
		{domain.SessionOpen, 2, "uploading", true},
		{domain.SessionCompleted, 3, "complete", true},
		{domain.SessionAborted, 0, "", false},
		{domain.SessionExpired, 1, "", false},
	} {
		got, err := domain.View(c.state, c.confirmed)
		if got != c.want || (err == nil) != c.ok {
			t.Errorf("View(%s, %d) = %q, %v; want %q, ok %v", c.state, c.confirmed, got, err, c.want, c.ok)
		}
	}
}

func TestFSPartETag(t *testing.T) {
	sum := sha256.Sum256([]byte("part"))
	got := domain.FSPartETag(sum)
	if len(got) != 66 || got[0] != '"' || got[65] != '"' || strings.ToLower(got) != got {
		t.Fatalf("FSPartETag = %s, want a quoted lowercase hex SHA-256", got)
	}
	if got[1:65] != hexOf(sum[:]) {
		t.Fatalf("FSPartETag = %s, want the hex of the digest", got)
	}
}

// hexOf is the lowercase hex of b.
func hexOf(b []byte) string {
	const digits = "0123456789abcdef"
	out := make([]byte, 0, 2*len(b))
	for _, c := range b {
		out = append(out, digits[c>>4], digits[c&0x0f])
	}
	return string(out)
}

func TestMissingRanges(t *testing.T) {
	for _, c := range []struct {
		count     int
		confirmed []int
		want      string
	}{
		{9, []int{1, 2, 3, 7, 8}, "4-6,9"},
		{3, nil, "1-3"},
		{3, []int{1, 2, 3}, ""},
		{5, []int{2, 4}, "1,3,5"},
		{1, nil, "1"},
		{6, []int{6, 1}, "2-5"},
	} {
		missing := domain.MissingParts(c.count, c.confirmed)
		if got := domain.Ranges(missing); got != c.want {
			t.Errorf("Ranges(MissingParts(%d, %v)) = %q, want %q", c.count, c.confirmed, got, c.want)
		}
	}
}
