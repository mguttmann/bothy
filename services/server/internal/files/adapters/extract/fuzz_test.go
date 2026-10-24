// SPDX-License-Identifier: Apache-2.0

package extract_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/extract"
	"github.com/OWNER/bothy/services/server/internal/files/adapters/extract/extracttest"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
)

// fuzzConfig are small limits, so the fuzzer reaches them.
var fuzzConfig = extract.Config{MaxTextBytes: 512, MaxUncompressedBytes: 1 << 16, MaxArchiveEntries: 20}

// validCodes is the closed set of design note 4.
var validCodes = map[domain.ExtractCode]bool{
	domain.CodeInputTooLarge: true, domain.CodeArchiveInvalid: true, domain.CodeArchiveEntriesExceeded: true,
	domain.CodeArchiveSizeExceeded: true, domain.CodeXMLDoctype: true, domain.CodeXMLInvalid: true,
	domain.CodeTimeout: true, domain.CodeObjectMissing: true,
}

// fuzzOne checks the contract: text within the limit, valid UTF-8 without NUL, or an
// *domain.ExtractError with a code of the set.
func fuzzOne(t *testing.T, mime string, data []byte) {
	r, err := extract.New(fuzzConfig)
	if err != nil {
		t.Fatal(err)
	}
	res, err := r.Extract(context.Background(), bytes.NewReader(data), mime)
	if err != nil {
		var xe *domain.ExtractError
		if !errors.As(err, &xe) || !validCodes[xe.Code] {
			t.Fatalf("error %v is no extraction code", err)
		}
		return
	}
	if int64(len(res.Text)) > fuzzConfig.MaxTextBytes || !utf8.ValidString(res.Text) || strings.ContainsRune(res.Text, 0) {
		t.Fatalf("result breaks the contract: %d bytes", len(res.Text))
	}
}

func FuzzExtractText(f *testing.F) {
	for _, s := range [][]byte{[]byte("plain"), {0xFF, 0xFE, 'a', 0}, {0xFE, 0xFF, 0xD8, 0x3D}, []byte("\xEF\xBB\xBFx\x00\xff")} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, data []byte) { fuzzOne(t, "text/plain", data) })
}

func FuzzExtractHTML(f *testing.F) {
	for _, s := range []string{"<p>a</p>", "<script>x</script>y", "<b>unclosed", "&amp;&#x1F600;&bogus;", "<!-- c --><style>"} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, data []byte) { fuzzOne(t, "text/html", data) })
}

func FuzzExtractOOXML(f *testing.F) {
	f.Add(extracttest.DOCX("Rechnung"))
	f.Add(extracttest.XLSX("a", "b"))
	f.Add(extracttest.PPTX("s"))
	f.Add(extracttest.XXESystem("http://127.0.0.1:1/x").DOCX())
	f.Add(extracttest.BombLyingHeader(100, 300))
	f.Fuzz(func(t *testing.T, data []byte) {
		for _, mt := range []string{extract.TypeDOCX, extract.TypeXLSX, extract.TypePPTX} {
			fuzzOne(t, mt, data)
		}
	})
}

func FuzzExtractODF(f *testing.F) {
	f.Add(extracttest.ODT("Text"))
	f.Add(extracttest.ODS("Zelle"))
	f.Add(extracttest.ODP("Seite"))
	f.Add(extracttest.UndefinedEntity.ODT())
	f.Fuzz(func(t *testing.T, data []byte) { fuzzOne(t, extract.TypeODT, data) })
}
