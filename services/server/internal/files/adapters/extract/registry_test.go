// SPDX-License-Identifier: Apache-2.0

package extract_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/extract"
	"github.com/OWNER/bothy/services/server/internal/files/adapters/extract/extracttest"
	filesmimetype "github.com/OWNER/bothy/services/server/internal/files/adapters/mimetype"
	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
)

// testConfig are limits large enough for every fixture.
var testConfig = extract.Config{MaxTextBytes: 1 << 16, MaxUncompressedBytes: 1 << 20, MaxArchiveEntries: 100, MaxXMLDepth: 64}

// Compile-time check: Registry is the outgoing port of the use case.
var _ app.TextExtractor = (*extract.Registry)(nil)

func newRegistry(t *testing.T, cfg extract.Config) *extract.Registry {
	t.Helper()
	r, err := extract.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func run(t *testing.T, cfg extract.Config, mime string, data []byte) (app.ExtractResult, error) {
	t.Helper()
	return newRegistry(t, cfg).Extract(context.Background(), bytes.NewReader(data), mime)
}

func mustText(t *testing.T, mime string, data []byte) string {
	t.Helper()
	res, err := run(t, testConfig, mime, data)
	if err != nil {
		t.Fatalf("%s: %v", mime, err)
	}
	return res.Text
}

func wantCode(t *testing.T, err error, code domain.ExtractCode) {
	t.Helper()
	var xe *domain.ExtractError
	if !errors.As(err, &xe) || xe.Code != code {
		t.Fatalf("error %v; want code %s", err, code)
	}
}

// detected asserts the type the merged detector reports for data, so a fixture the detector
// misreads fails loudly.
func detected(t *testing.T, data []byte, want string) {
	t.Helper()
	got := domain.MediaType(filesmimetype.NewDetector(1 << 16).Detect(data))
	if got != want {
		t.Fatalf("detected %q; want %q", got, want)
	}
}

func TestVersions(t *testing.T) {
	t.Parallel()
	r := newRegistry(t, testConfig)
	cases := map[string]string{
		"text/plain": extract.VersionText, "text/markdown": extract.VersionText, "text/csv": extract.VersionText,
		"application/json": extract.VersionText, "text/x-python": extract.VersionText, "text/plain; charset=utf-8": extract.VersionText,
		"text/html": extract.VersionHTML, "text/rtf": "", "image/png": "", "application/zip": "", "application/pdf": "",
		"application/octet-stream": "",
		extract.TypeDOCX:           extract.VersionOOXML, extract.TypeXLSX: extract.VersionOOXML, extract.TypePPTX: extract.VersionOOXML,
		extract.TypeODT: extract.VersionODF, extract.TypeODS: extract.VersionODF, extract.TypeODP: extract.VersionODF,
	}
	form := regexp.MustCompile(`^builtin-[a-z]+/[1-9][0-9]*$`)
	for mime, want := range cases {
		v, ok := r.Version(mime)
		if v != want || ok != (want != "") {
			t.Errorf("%s = %q, %v; want %q", mime, v, ok, want)
		}
		if ok && !form.MatchString(v) {
			t.Errorf("%s: identifier %q is not <adapter>/<n>", mime, v)
		}
	}
	if _, err := r.Extract(context.Background(), strings.NewReader("x"), "image/png"); err == nil {
		t.Fatal("unsupported type extracted")
	}
	if _, err := extract.New(extract.Config{}); err == nil {
		t.Fatal("zero limits accepted")
	}
}

func TestDetectedTypes(t *testing.T) {
	t.Parallel()
	detected(t, extracttest.DOCX("x"), extract.TypeDOCX)
	detected(t, extracttest.XLSX("x"), extract.TypeXLSX)
	detected(t, extracttest.PPTX("x"), extract.TypePPTX)
	detected(t, extracttest.ODT("x"), extract.TypeODT)
	detected(t, extracttest.ODS("x"), extract.TypeODS)
	detected(t, extracttest.ODP("x"), extract.TypeODP)
}

// countingReader counts the bytes read from it.
type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

func TestBuiltinText(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, mime string
		in         []byte
		want       string
	}{
		{"utf8", "text/plain", []byte("Grüße aus Berlin"), "Grüße aus Berlin"},
		{"utf8 bom", "text/plain", []byte("\xEF\xBB\xBFRechnung"), "Rechnung"},
		{"utf16le bom", "text/plain", []byte{0xFF, 0xFE, 'R', 0, 0xFC, 0, 0x3D, 0xD8, 0x00, 0xDE}, "Rü😀"},
		{"utf16be bom", "text/plain", []byte{0xFE, 0xFF, 0, 'R', 0, 0xFC}, "Rü"},
		{"utf16 unpaired and odd", "text/plain", []byte{0xFF, 0xFE, 0x3D, 0xD8, 'a', 0, 'b'}, "�a�"},
		{"invalid bytes", "text/plain", []byte("a\xff\xfeb"), "a��b"},
		{"nul", "text/plain", []byte("a\x00b"), "a b"},
		{"csv", "text/csv", []byte("name,amount\nAlex,12\n"), "name,amount\nAlex,12\n"},
		{"json", "application/json", []byte(`{"who":"Sam"}`), `{"who":"Sam"}`},
		{"python", "text/x-python", []byte("def hello():\n    return 1\n"), "def hello():\n    return 1\n"},
	}
	for _, c := range cases {
		if got := mustText(t, c.mime, c.in); got != c.want {
			t.Errorf("%s: %q; want %q", c.name, got, c.want)
		}
	}
	t.Run("truncation", func(t *testing.T) {
		t.Parallel()
		cfg := testConfig
		cfg.MaxTextBytes = 10
		cr := &countingReader{r: strings.NewReader(strings.Repeat("abcdefghij", 10000))}
		res, err := newRegistry(t, cfg).Extract(context.Background(), cr, "text/plain")
		if err != nil || res.Text != "abcdefghij" || !res.Truncated {
			t.Fatalf("%q %v %v", res.Text, res.Truncated, err)
		}
		// One bufio buffer at most beyond what the builder needed.
		if cr.n > 10+4096 {
			t.Fatalf("read %d bytes", cr.n)
		}
		res, err = newRegistry(t, cfg).Extract(context.Background(), strings.NewReader("abcdefghij"), "text/plain")
		if err != nil || res.Truncated {
			t.Fatalf("exact fit: %v %v", res.Truncated, err)
		}
	})
	t.Run("reader error", func(t *testing.T) {
		t.Parallel()
		boom := errors.New("boom")
		_, err := newRegistry(t, testConfig).Extract(context.Background(), io.MultiReader(strings.NewReader("ab"), errReader{boom}), "text/plain")
		if !errors.Is(err, boom) {
			t.Fatalf("%v", err)
		}
	})
}

type errReader struct{ err error }

func (e errReader) Read([]byte) (int, error) { return 0, e.err }

func TestBuiltinHTML(t *testing.T) {
	t.Parallel()
	in := `<!DOCTYPE html><html><head><title>Plan</title><style>.x{color:red}</style>` +
		`<script>var secretScript = 1;</script></head><body><!-- hidden comment -->` +
		`<p>Rech<b>nung</b> &amp; Vertrag</p><div>zweiter</div>Teil<p>kaputt <i>ohne Ende`
	got := mustText(t, "text/html", []byte(in))
	for _, absent := range []string{"secretScript", "color", "hidden comment", "DOCTYPE"} {
		if strings.Contains(got, absent) {
			t.Errorf("%q in %q", absent, got)
		}
	}
	for _, present := range []string{"Plan", "Rechnung & Vertrag", "zweiter\nTeil", "kaputt ohne Ende"} {
		if !strings.Contains(got, present) {
			t.Errorf("%q missing in %q", present, got)
		}
	}
}

func TestDOCX(t *testing.T) {
	t.Parallel()
	body := `<w:p><w:r><w:t>Eins</w:t><w:tab/><w:t>Zwei</w:t><w:br/><w:t>Drei</w:t></w:r></w:p>` +
		`<w:p><w:del><w:r><w:delText>geloescht</w:delText></w:r></w:del><w:r><w:instrText>PAGE</w:instrText><w:t>Vier</w:t></w:r></w:p>`
	got := mustText(t, extract.TypeDOCX, extracttest.DOCXWith(extracttest.DOCXDocument(body)))
	if got != "Eins\tZwei\nDrei\nVier" {
		t.Fatalf("%q", got)
	}
	if got := mustText(t, extract.TypeDOCX, extracttest.DOCXWith(extracttest.DOCXStrictDocument(extracttest.Paragraphs("streng")))); got != "streng" {
		t.Fatalf("strict: %q", got)
	}
	_, err := run(t, testConfig, extract.TypeDOCX, extracttest.Zip(extracttest.Entry{Name: "word/other.xml", Data: []byte("<a/>")}))
	wantCode(t, err, domain.CodeArchiveInvalid)
}

func TestXLSX(t *testing.T) {
	t.Parallel()
	sst := extracttest.XMLHeader + `<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
		`<si><t>Kunde</t></si><si><r><t>Bet</t></r><r><t>rag</t></r><rPh><t>phonetisch</t></rPh></si><si><t>Ende</t></si></sst>`
	if got := mustText(t, extract.TypeXLSX, extracttest.XLSXWith(sst)); got != "Kunde\nBetrag\nEnde" {
		t.Fatalf("%q", got)
	}
	if got := mustText(t, extract.TypeXLSX, extracttest.XLSXWith("")); got != "" {
		t.Fatalf("no shared strings: %q", got)
	}
}

func TestPPTX(t *testing.T) {
	t.Parallel()
	slides := map[int]string{}
	order := []int{}
	for i := 1; i <= 10; i++ {
		slides[i] = extracttest.Slide("Folie", strings.Repeat("x", i))
		order = append(order, 11-i)
	}
	got := mustText(t, extract.TypePPTX, extracttest.PPTXWith(slides, order))
	lines := strings.Split(got, "\n")
	if len(lines) != 20 || lines[3] != "xx" || lines[19] != strings.Repeat("x", 10) {
		t.Fatalf("%q", got)
	}
}

func TestODF(t *testing.T) {
	t.Parallel()
	body := `<office:text><text:tracked-changes><text:changed-region><text:deletion><text:p>weg</text:p></text:deletion></text:changed-region></text:tracked-changes>` +
		`<text:h>Titel</text:h><text:p>a<text:s/>b<text:tab/>c<text:line-break/>d<office:annotation><text:p>Kommentar</text:p></office:annotation></text:p></office:text>`
	if got := mustText(t, extract.TypeODT, extracttest.ODFWith(extracttest.MimeODT, extracttest.ODFContent(body))); got != "Titel\na b\tc\nd" {
		t.Fatalf("odt: %q", got)
	}
	if got := mustText(t, extract.TypeODS, extracttest.ODS("Zelle", "zwei")); got != "Zelle\nzwei" {
		t.Fatalf("ods: %q", got)
	}
	if got := mustText(t, extract.TypeODP, extracttest.ODP("Seite eins", "Seite zwei")); got != "Seite eins\nSeite zwei" {
		t.Fatalf("odp: %q", got)
	}
	_, err := run(t, testConfig, extract.TypeODT, extracttest.ODFWith(extracttest.MimeODT, ""))
	wantCode(t, err, domain.CodeArchiveInvalid)
}

func TestZipLimitsPublic(t *testing.T) {
	t.Parallel()
	cfg := extract.Config{MaxTextBytes: 1 << 16, MaxUncompressedBytes: 4096, MaxArchiveEntries: 10, MaxXMLDepth: 64}
	_, err := run(t, cfg, extract.TypeDOCX, extracttest.TooManyEntries(20))
	wantCode(t, err, domain.CodeArchiveEntriesExceeded)
	_, err = run(t, cfg, extract.TypeDOCX, extracttest.BombDeclared(1<<20))
	wantCode(t, err, domain.CodeArchiveSizeExceeded)
	_, err = run(t, cfg, extract.TypeDOCX, extracttest.BombLyingHeader(1000, 3000))
	wantCode(t, err, domain.CodeArchiveInvalid)
	for name, data := range map[string][]byte{
		"encrypted": extracttest.Encrypted(), "method": extracttest.UnknownMethod(), "duplicate": extracttest.DuplicatePart(),
		"not a zip": []byte("PK but not really a zip"),
	} {
		_, err := run(t, cfg, extract.TypeDOCX, data)
		var xe *domain.ExtractError
		if !errors.As(err, &xe) || xe.Code != domain.CodeArchiveInvalid {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestXMLDoctypeRejected(t *testing.T) {
	t.Parallel()
	variants := map[string]struct {
		v    extracttest.XMLVariant
		code domain.ExtractCode
	}{
		"http":      {extracttest.XXESystem("http://127.0.0.1:1/x"), domain.CodeXMLDoctype},
		"file":      {extracttest.XXESystem("file:///etc/hostname"), domain.CodeXMLDoctype},
		"parameter": {extracttest.ParameterEntity, domain.CodeXMLDoctype},
		"expansion": {extracttest.InternalExpansion, domain.CodeXMLDoctype},
		"plain":     {extracttest.PlainDoctype, domain.CodeXMLDoctype},
		"undefined": {extracttest.UndefinedEntity, domain.CodeXMLInvalid},
		"non-utf8":  {extracttest.NonUTF8, domain.CodeXMLInvalid},
	}
	for name, c := range variants {
		_, err := run(t, testConfig, extract.TypeDOCX, c.v.DOCX())
		var xe *domain.ExtractError
		if !errors.As(err, &xe) || xe.Code != c.code {
			t.Errorf("docx %s: %v; want %s", name, err, c.code)
		}
		_, err = run(t, testConfig, extract.TypeODT, c.v.ODT())
		if !errors.As(err, &xe) || xe.Code != c.code {
			t.Errorf("odt %s: %v; want %s", name, err, c.code)
		}
	}
}

// stallingReader blocks until the test ends.
type stallingReader struct{ release chan struct{} }

func (s stallingReader) Read([]byte) (int, error) {
	<-s.release
	return 0, io.EOF
}

func TestExtractContextDeadline(t *testing.T) {
	t.Parallel()
	r := newRegistry(t, testConfig)
	for _, mime := range []string{"text/plain", "text/html", extract.TypeDOCX, extract.TypeODT} {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := r.Extract(ctx, strings.NewReader("abc"), mime); !errors.Is(err, context.Canceled) {
			t.Errorf("%s passed: %v", mime, err)
		}
		release := make(chan struct{})
		ctx, cancel = context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() {
			_, err := r.Extract(ctx, stallingReader{release}, mime)
			done <- err
		}()
		cancel()
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Errorf("%s stalling: %v", mime, err)
		}
		close(release)
	}
}

func TestExtractImportsNoNetwork(t *testing.T) {
	t.Parallel()
	out, err := exec.CommandContext(context.Background(), "go", "list", "-deps", ".").Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, dep := range strings.Fields(string(out)) {
		if dep == "net" || dep == "net/http" || dep == "os/exec" || strings.HasPrefix(dep, "net/") {
			t.Errorf("the extract package depends on %s", dep)
		}
	}
}

func TestResultIsCleanText(t *testing.T) {
	t.Parallel()
	res, err := run(t, testConfig, "text/plain", []byte("x\x00\xffy"))
	if err != nil || !utf8.ValidString(res.Text) || strings.ContainsRune(res.Text, 0) || res.Version != extract.VersionText {
		t.Fatalf("%q %v", res.Text, err)
	}
}
