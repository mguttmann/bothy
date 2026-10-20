// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/files/domain"
)

func TestParseVariant(t *testing.T) {
	for _, c := range []struct {
		in    string
		want  domain.Variant
		ok    bool
		thumb bool
	}{
		{"original", domain.VariantOriginal, true, false},
		{"small", domain.VariantSmall, true, true},
		{"medium", domain.VariantMedium, true, true},
		{"large", domain.VariantLarge, true, true},
		{"poster", domain.VariantPoster, true, true},
		{"", "", false, false},
		{"Small", "", false, false},
		{"orig-1", "", false, false},
		{" small", "", false, false},
	} {
		got, ok := domain.ParseVariant(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("ParseVariant(%q) = %q, %v; want %q, %v", c.in, got, ok, c.want, c.ok)
		}
		if ok && got.IsThumbnail() != c.thumb {
			t.Errorf("%q.IsThumbnail() = %v", got, !c.thumb)
		}
	}
	if domain.Variant("bogus").IsThumbnail() {
		t.Error("an unknown variant is no thumbnail")
	}
}

func TestReadiness_Table(t *testing.T) {
	const d, n, b = domain.Deliverable, domain.NotReady, domain.Blocked
	// Columns: scan_status pending, clean, infected, error, skipped.
	scans := []string{"pending", "clean", "infected", "error", "skipped"}
	for up, row := range map[string][5]domain.Readiness{
		"pending":  {n, n, b, b, n},
		"complete": {n, d, b, b, d},
		"failed":   {b, b, b, b, b},
	} {
		for i, scan := range scans {
			if got := domain.ReadinessOf(up, scan); got != row[i] {
				t.Errorf("ReadinessOf(%s, %s) = %d, want %d", up, scan, got, row[i])
			}
		}
	}
	for _, k := range [][2]string{{"", ""}, {"complete", ""}, {"Complete", "clean"}, {"complete", "CLEAN"}} {
		if got := domain.ReadinessOf(k[0], k[1]); got != domain.NotReady {
			t.Errorf("ReadinessOf(%q, %q) = %d, want NotReady", k[0], k[1], got)
		}
	}
	if domain.ReadinessInvalidZero != 0 || domain.ReadinessInvalidZero == domain.Deliverable {
		t.Error("the zero value must not mean deliverable")
	}
}

func TestPresentation_InlineList(t *testing.T) {
	const octet = "application/octet-stream"
	for _, c := range []struct {
		mime   string
		v      domain.Variant
		ct     string
		inline bool
	}{
		{"image/jpeg", domain.VariantOriginal, "image/jpeg", true},
		{"image/png", domain.VariantOriginal, "image/png", true},
		{"image/gif", domain.VariantOriginal, "image/gif", true},
		{"audio/mp4", domain.VariantOriginal, "audio/mp4", true},
		{"audio/mpeg", domain.VariantOriginal, "audio/mpeg", true},
		{"video/mp4", domain.VariantOriginal, "video/mp4", true},
		{"IMAGE/PNG", domain.VariantOriginal, "image/png", true},
		{"image/jpeg; charset=utf-8", domain.VariantOriginal, "image/jpeg", true},
		{" video/mp4 ;codecs=avc1", domain.VariantOriginal, "video/mp4", true},
		{"text/html", domain.VariantOriginal, octet, false},
		{"text/html; charset=utf-8", domain.VariantOriginal, octet, false},
		{"image/svg+xml", domain.VariantOriginal, octet, false},
		{"application/pdf", domain.VariantOriginal, octet, false},
		{"image/webp", domain.VariantOriginal, octet, false},
		{"image/heic", domain.VariantOriginal, octet, false},
		{"text/plain", domain.VariantOriginal, octet, false},
		{"application/x-unknown", domain.VariantOriginal, octet, false},
		{"", domain.VariantOriginal, octet, false},
		{"image/jpeg", domain.VariantSmall, "image/jpeg", true},
		{"image/png", domain.VariantMedium, "image/png", true},
		{"image/jpeg", domain.VariantLarge, "image/jpeg", true},
		{"image/jpeg", domain.VariantPoster, "image/jpeg", true},
		{"Image/JPEG; q=1", domain.VariantSmall, "image/jpeg", true},
		{"image/gif", domain.VariantSmall, octet, false},
		{"image/svg+xml", domain.VariantSmall, octet, false},
		{"text/html", domain.VariantPoster, octet, false},
		{"", domain.VariantSmall, octet, false},
		{"image/jpeg", domain.Variant(""), octet, false},
		{"image/jpeg", domain.Variant("bogus"), octet, false},
	} {
		ct, inline := domain.Presentation(c.mime, c.v)
		if ct != c.ct || inline != c.inline {
			t.Errorf("Presentation(%q, %q) = %q, %v; want %q, %v", c.mime, c.v, ct, inline, c.ct, c.inline)
		}
	}
}

func TestContentDisposition_Table(t *testing.T) {
	for _, c := range []struct {
		name   string
		inline bool
		want   string
	}{
		{"photo.png", true, `inline; filename="photo.png"; filename*=UTF-8''photo.png`},
		{"Résumé for Sam.pdf", false, `attachment; filename="R_sum_ for Sam.pdf"; filename*=UTF-8''R%C3%A9sum%C3%A9%20for%20Sam.pdf`},
		{`say "hi".txt`, false, `attachment; filename="say _hi_.txt"; filename*=UTF-8''say%20%22hi%22.txt`},
		{`back\slash.txt`, false, `attachment; filename="back_slash.txt"; filename*=UTF-8''back%5Cslash.txt`},
		{"100%.txt", false, `attachment; filename="100_.txt"; filename*=UTF-8''100%25.txt`},
		{"a;b.txt", false, `attachment; filename="a_b.txt"; filename*=UTF-8''a%3Bb.txt`},
		{"invoice\u202Efdp.exe", false, `attachment; filename="invoice_fdp.exe"; filename*=UTF-8''invoice%E2%80%AEfdp.exe`},
		{"a\r\nb\x00c\x7f.txt", false, `attachment; filename="a__b_c_.txt"; filename*=UTF-8''a%0D%0Ab%00c%7F.txt`},
		{"!#$&+-.^_`|~", true, "inline; filename=\"!#$&+-.^_`|~\"; filename*=UTF-8''!#$&+-.^_`|~"},
		{"x'(),/:<=>?@[]{}*", false, `attachment; filename="x'(),/:<=>?@[]{}*"; filename*=UTF-8''x%27%28%29%2C%2F%3A%3C%3D%3E%3F%40%5B%5D%7B%7D%2A`},
		{"\xff.txt", false, `attachment; filename="_.txt"; filename*=UTF-8''%FF.txt`},
		{"", false, `attachment; filename="file"; filename*=UTF-8''file`},
		{"", true, `inline; filename="file"; filename*=UTF-8''file`},
	} {
		if got := domain.ContentDisposition(c.name, c.inline); got != c.want {
			t.Errorf("ContentDisposition(%q, %v)\n got %s\nwant %s", c.name, c.inline, got, c.want)
		}
	}
}

func FuzzContentDisposition(f *testing.F) {
	for _, s := range []string{"", "photo.png", "Résumé for Sam.pdf", `"\%;`, "\u202E\r\n\x00", "\xff\xfe", "日本語.txt"} {
		f.Add(s, true)
		f.Add(s, false)
	}
	f.Fuzz(func(t *testing.T, name string, inline bool) {
		got := domain.ContentDisposition(name, inline)
		if strings.ContainsAny(got, "\r\n") {
			t.Fatalf("output of %d bytes holds CR or LF", len(got))
		}
		for i := 0; i < len(got); i++ {
			if got[i] < 0x20 || got[i] > 0x7e {
				t.Fatalf("byte %#x outside printable ASCII at %d", got[i], i)
			}
		}
		prefix := "attachment; filename=\""
		if inline {
			prefix = "inline; filename=\""
		}
		if !strings.HasPrefix(got, prefix) {
			t.Fatalf("output does not start with %q", prefix)
		}
		rest := got[len(prefix):]
		end := strings.IndexByte(rest, '"')
		if end < 0 {
			t.Fatal("quoted string is not closed")
		}
		if strings.ContainsAny(rest[:end], `\%;`) {
			t.Fatal("fallback holds a backslash, percent sign or semicolon")
		}
		ext := rest[end+1:]
		if !strings.HasPrefix(ext, "; filename*=UTF-8''") {
			t.Fatal("filename* does not follow filename")
		}
		enc := strings.TrimPrefix(ext, "; filename*=UTF-8''")
		if strings.ContainsAny(enc, "\"; ") {
			t.Fatal("encoded value holds a quote, semicolon or space")
		}
	})
}

func TestThumbnailName(t *testing.T) {
	for _, c := range []struct {
		name string
		v    domain.Variant
		mime string
		want string
	}{
		{"Beach day.heic", domain.VariantSmall, "image/jpeg", "Beach day-small.jpg"},
		{"Beach day.heic", domain.VariantMedium, "image/png", "Beach day-medium.png"},
		{"diagram.v2.png", domain.VariantLarge, "image/png", "diagram.v2-large.png"},
		{"clip.mp4", domain.VariantPoster, "image/jpeg", "clip-poster.jpg"},
		{"README", domain.VariantSmall, "image/jpeg", "README-small.jpg"},
		{".hidden", domain.VariantSmall, "image/jpeg", ".hidden-small.jpg"},
		{"name.", domain.VariantSmall, "image/jpeg", "name-small.jpg"},
		{"Résumé.pdf", domain.VariantSmall, "IMAGE/PNG; x=y", "Résumé-small.png"},
		{"photo.jpg", domain.VariantSmall, "image/gif", "photo-small.jpg"},
	} {
		if got := domain.ThumbnailName(c.name, c.v, c.mime); got != c.want {
			t.Errorf("ThumbnailName(%q, %q, %q) = %q, want %q", c.name, c.v, c.mime, got, c.want)
		}
	}
}
