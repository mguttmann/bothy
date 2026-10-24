// SPDX-License-Identifier: Apache-2.0

package extract_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/extract"
	"github.com/OWNER/bothy/services/server/internal/files/adapters/extract/extracttest"
)

var update = flag.Bool("update", false, "rewrite testdata/golden/<adapter>.txt; refused under an unchanged version")

// goldenFixture is one input of the fixed fixture set of an adapter.
type goldenFixture struct {
	mime string
	data []byte
}

// goldenSets are the fixed fixtures per adapter name (the part before the slash of the version).
func goldenSets() map[string][]goldenFixture {
	return map[string][]goldenFixture{
		"builtin-text": {
			{"text/plain", []byte("Grüße\x00 aus\xff Berlin\r\n\tzweite Zeile")},
			{"text/plain", []byte{0xFF, 0xFE, 'R', 0, 0xFC, 0, 0x3D, 0xD8, 0x00, 0xDE}},
			{"text/csv", []byte("name,amount\nAlex,12\nSam,7\n")},
			{"application/json", []byte(`{"who":"Robin","items":[1,2,3]}`)},
		},
		"builtin-html": {
			{"text/html", []byte(`<!DOCTYPE html><title>Plan</title><script>x()</script><p>Rech<b>nung</b> &amp; <i>Vertrag</i></p><ul><li>eins<li>zwei</ul>`)},
		},
		"builtin-ooxml": {
			{extract.TypeDOCX, extracttest.DOCXWith(extracttest.DOCXDocument(`<w:p><w:r><w:t>Eins</w:t><w:tab/><w:t>Zwei</w:t><w:br/><w:delText>weg</w:delText></w:r></w:p>` +
				extracttest.Paragraphs("Die Rechnung für den Lieferantenvertrag ist fertig.")))},
			{extract.TypeXLSX, extracttest.XLSX("Kunde", "Betrag", "Summe")},
			{extract.TypePPTX, extracttest.PPTX("Folie eins", "Folie zwei")},
		},
		"builtin-odf": {
			{extract.TypeODT, extracttest.ODFWith(extracttest.MimeODT, extracttest.ODFContent(`<office:text><text:h>Titel</text:h><text:p>a<text:s/>b<text:tab/>c<office:annotation><text:p>k</text:p></office:annotation></text:p></office:text>`))},
			{extract.TypeODS, extracttest.ODS("Zelle", "zwei")},
			{extract.TypeODP, extracttest.ODP("Seite eins", "Seite zwei")},
		},
	}
}

// TestExtractorGolden holds each adapter's output hash over its fixed fixtures against
// testdata/golden/<adapter>.txt (brief M3-15 design note 3). A changed output under an unchanged
// version fails; -update writes a new hash only when the version was raised.
func TestExtractorGolden(t *testing.T) {
	reg := newRegistry(t, testConfig)
	for name, set := range goldenSets() {
		h := sha256.New()
		version := ""
		for _, f := range set {
			res, err := reg.Extract(context.Background(), bytes.NewReader(f.data), f.mime)
			if err != nil {
				t.Fatalf("%s %s: %v", name, f.mime, err)
			}
			if !strings.HasPrefix(res.Version, name+"/") {
				t.Fatalf("%s: version %q", name, res.Version)
			}
			version = res.Version
			_, _ = fmt.Fprintf(h, "%s\x00%t\x00%s\x00", f.mime, res.Truncated, res.Text)
		}
		got := hex.EncodeToString(h.Sum(nil))
		path := filepath.Join("testdata", "golden", name+".txt")
		raw, err := os.ReadFile(path) //nolint:gosec // G304: a fixed test path
		var goldVersion, goldHash string
		if err == nil {
			_, _ = fmt.Sscanf(string(raw), "version %s\nsha256 %s\n", &goldVersion, &goldHash)
		}
		if goldHash == got && goldVersion == version {
			continue
		}
		if !*update {
			t.Errorf("%s: output hash %s under %s differs from the golden (%s under %s): raise the version of the adapter and run go test -run TestExtractorGolden -update",
				name, got, version, goldHash, goldVersion)
			continue
		}
		if goldVersion == version && goldHash != "" {
			t.Errorf("%s: the output changed under the unchanged version %s; raise the version first", name, version)
			continue
		}
		if err := os.WriteFile(path, []byte(fmt.Sprintf("version %s\nsha256 %s\n", version, got)), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}
