// SPDX-License-Identifier: Apache-2.0

// Package extracttest builds the test files of the text extraction at test time (brief M3-15
// design note 14): DOCX, XLSX, PPTX, ODT, ODS and ODP with given texts, the zip bomb variants and
// the XML variants with document type declarations. No binary fixture lives in the repository.
// The containers are written in the entry order Office and LibreOffice use ([Content_Types].xml,
// _rels/.rels, then the parts; for ODF a stored mimetype entry first), so the type detection of
// adapters/mimetype recognises them; tests that rely on detection assert the detected type first.
//
// Example:
//
//	doc := extracttest.DOCX("Die Rechnung ist fertig.")
package extracttest

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"strings"
)

// Entry is one entry of a zip archive Zip writes.
type Entry struct {
	// Name is the entry name.
	Name string
	// Data are the uncompressed bytes.
	Data []byte
	// Store writes the entry without compression (else deflate).
	Store bool
	// Raw writes Data as the already compressed body with the header values below
	// (zip.Writer.CreateRaw): Method, DeclaredSize, CRC32 and Flags.
	Raw          bool
	Method       uint16
	DeclaredSize uint64
	CRC32        uint32
	Flags        uint16
}

// Zip writes entries in order into a zip archive. It panics on an error of the writer, which
// only a bug in a test can cause.
func Zip(entries ...Entry) []byte {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, e := range entries {
		if e.Raw {
			fw, err := w.CreateRaw(&zip.FileHeader{Name: e.Name, Method: e.Method, Flags: e.Flags, CRC32: e.CRC32,
				CompressedSize64: uint64(len(e.Data)), UncompressedSize64: e.DeclaredSize})
			must(err)
			_, err = fw.Write(e.Data)
			must(err)
			continue
		}
		method := zip.Deflate
		if e.Store {
			method = zip.Store
		}
		fw, err := w.CreateHeader(&zip.FileHeader{Name: e.Name, Method: method})
		must(err)
		_, err = fw.Write(e.Data)
		must(err)
	}
	must(w.Close())
	return buf.Bytes()
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

// Escape escapes s for XML character data.
func Escape(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '&':
			b.WriteString("&amp;")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// The fixed package parts of OOXML.
const (
	partContentTypes = "[Content_Types].xml"
	partRels         = "_rels/.rels"
	partDocument     = "word/document.xml"
)

// XMLHeader is the declaration every generated part starts with.
const XMLHeader = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n"

// The namespaces of the generated parts.
const (
	nsW      = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
	nsWStr   = "http://purl.oclc.org/ooxml/wordprocessingml/main"
	nsS      = "http://schemas.openxmlformats.org/spreadsheetml/2006/main"
	nsA      = "http://schemas.openxmlformats.org/drawingml/2006/main"
	nsP      = "http://schemas.openxmlformats.org/presentationml/2006/main"
	nsOffice = "urn:oasis:names:tc:opendocument:xmlns:office:1.0"
	nsText   = "urn:oasis:names:tc:opendocument:xmlns:text:1.0"
	nsTable  = "urn:oasis:names:tc:opendocument:xmlns:table:1.0"
	nsDraw   = "urn:oasis:names:tc:opendocument:xmlns:drawing:1.0"
)

// contentTypes is [Content_Types].xml with the main content type of the format.
func contentTypes(overrides ...string) []byte {
	var b strings.Builder
	b.WriteString(XMLHeader + `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
		`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
		`<Default Extension="xml" ContentType="application/xml"/>`)
	for i := 0; i+1 < len(overrides); i += 2 {
		fmt.Fprintf(&b, `<Override PartName="/%s" ContentType="%s"/>`, overrides[i], overrides[i+1])
	}
	b.WriteString(`</Types>`)
	return []byte(b.String())
}

// rels is _rels/.rels pointing at the main part.
func rels(target string) []byte {
	return []byte(XMLHeader + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="` +
		target + `"/></Relationships>`)
}

// DOCXDocument returns word/document.xml whose body holds bodyXML, in the transitional namespace.
func DOCXDocument(bodyXML string) string {
	return XMLHeader + `<w:document xmlns:w="` + nsW + `"><w:body>` + bodyXML + `</w:body></w:document>`
}

// DOCXStrictDocument is DOCXDocument in the strict OOXML namespace.
func DOCXStrictDocument(bodyXML string) string {
	return XMLHeader + `<w:document xmlns:w="` + nsWStr + `"><w:body>` + bodyXML + `</w:body></w:document>`
}

// Paragraphs returns one w:p with one run per text.
func Paragraphs(texts ...string) string {
	var b strings.Builder
	for _, t := range texts {
		b.WriteString(`<w:p><w:r><w:t xml:space="preserve">` + Escape(t) + `</w:t></w:r></w:p>`)
	}
	return b.String()
}

// DOCXWith writes a DOCX container with the given word/document.xml; extra entries follow it.
func DOCXWith(documentXML string, extra ...Entry) []byte {
	entries := []Entry{
		{Name: partContentTypes, Data: contentTypes(partDocument,
			"application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml")},
		{Name: partRels, Data: rels(partDocument)},
		{Name: partDocument, Data: []byte(documentXML)},
	}
	return Zip(append(entries, extra...)...)
}

// DOCX writes a DOCX with one paragraph per text.
func DOCX(paragraphs ...string) []byte { return DOCXWith(DOCXDocument(Paragraphs(paragraphs...))) }

// SharedStrings returns xl/sharedStrings.xml with one si per string.
func SharedStrings(strs ...string) string {
	var b strings.Builder
	b.WriteString(XMLHeader + `<sst xmlns="` + nsS + `" count="` + fmt.Sprint(len(strs)) + `">`)
	for _, s := range strs {
		b.WriteString(`<si><t xml:space="preserve">` + Escape(s) + `</t></si>`)
	}
	b.WriteString(`</sst>`)
	return b.String()
}

// XLSXWith writes an XLSX container; sharedStrings "" leaves out xl/sharedStrings.xml.
func XLSXWith(sharedStrings string) []byte {
	sheet := XMLHeader + `<worksheet xmlns="` + nsS + `"><sheetData><row r="1"><c r="A1"><v>42</v></c></row></sheetData></worksheet>`
	wb := XMLHeader + `<workbook xmlns="` + nsS + `" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
		`<sheets><sheet name="Sheet1" sheetId="1" r:id="rId1"/></sheets></workbook>`
	entries := []Entry{
		{Name: partContentTypes, Data: contentTypes("xl/workbook.xml",
			"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml")},
		{Name: partRels, Data: rels("xl/workbook.xml")},
		{Name: "xl/workbook.xml", Data: []byte(wb)},
		{Name: "xl/worksheets/sheet1.xml", Data: []byte(sheet)},
	}
	if sharedStrings != "" {
		entries = append(entries, Entry{Name: "xl/sharedStrings.xml", Data: []byte(sharedStrings)})
	}
	return Zip(entries...)
}

// XLSX writes an XLSX whose shared strings are strs.
func XLSX(strs ...string) []byte { return XLSXWith(SharedStrings(strs...)) }

// Slide returns ppt/slides/slide<N>.xml content with one paragraph per text.
func Slide(texts ...string) string {
	var b strings.Builder
	b.WriteString(XMLHeader + `<p:sld xmlns:a="` + nsA + `" xmlns:p="` + nsP + `"><p:cSld><p:spTree><p:sp><p:txBody>`)
	for _, t := range texts {
		b.WriteString(`<a:p><a:r><a:t>` + Escape(t) + `</a:t></a:r></a:p>`)
	}
	b.WriteString(`</p:txBody></p:sp></p:spTree></p:cSld></p:sld>`)
	return b.String()
}

// PPTXWith writes a PPTX with the given slide parts, keyed by slide number.
func PPTXWith(slides map[int]string, order []int) []byte {
	pres := XMLHeader + `<p:presentation xmlns:p="` + nsP + `"/>`
	entries := []Entry{
		{Name: partContentTypes, Data: contentTypes("ppt/presentation.xml",
			"application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml")},
		{Name: partRels, Data: rels("ppt/presentation.xml")},
		{Name: "ppt/presentation.xml", Data: []byte(pres)},
	}
	for _, n := range order {
		entries = append(entries, Entry{Name: fmt.Sprintf("ppt/slides/slide%d.xml", n), Data: []byte(slides[n])})
	}
	return Zip(entries...)
}

// PPTX writes a PPTX with one slide per text, numbered from 1.
func PPTX(slideTexts ...string) []byte {
	slides := map[int]string{}
	order := []int{}
	for i, t := range slideTexts {
		slides[i+1] = Slide(t)
		order = append(order, i+1)
	}
	return PPTXWith(slides, order)
}

// The ODF media types (the stored mimetype entry).
const (
	MimeODT = "application/vnd.oasis.opendocument.text"
	MimeODS = "application/vnd.oasis.opendocument.spreadsheet"
	MimeODP = "application/vnd.oasis.opendocument.presentation"
)

// ODFContent returns content.xml whose office:body holds bodyXML.
func ODFContent(bodyXML string) string {
	return XMLHeader + `<office:document-content xmlns:office="` + nsOffice + `" xmlns:text="` + nsText +
		`" xmlns:table="` + nsTable + `" xmlns:draw="` + nsDraw + `" office:version="1.3">` +
		`<office:automatic-styles/><office:body>` + bodyXML + `</office:body></office:document-content>`
}

// ODFWith writes an ODF container of mimeType with the given content.xml ("" leaves it out).
func ODFWith(mimeType, contentXML string) []byte {
	entries := []Entry{{Name: "mimetype", Data: []byte(mimeType), Store: true}}
	if contentXML != "" {
		entries = append(entries, Entry{Name: "content.xml", Data: []byte(contentXML)})
	}
	entries = append(entries,
		Entry{Name: "styles.xml", Data: []byte(XMLHeader + `<office:document-styles xmlns:office="` + nsOffice + `"/>`)},
		Entry{Name: "META-INF/manifest.xml", Data: []byte(XMLHeader +
			`<manifest:manifest xmlns:manifest="urn:oasis:names:tc:opendocument:xmlns:manifest:1.0">` +
			`<manifest:file-entry manifest:full-path="/" manifest:media-type="` + mimeType + `"/></manifest:manifest>`)})
	return Zip(entries...)
}

// ODTBody returns the office:text body with one text:p per paragraph.
func ODTBody(paragraphs ...string) string {
	var b strings.Builder
	b.WriteString(`<office:text>`)
	for _, p := range paragraphs {
		b.WriteString(`<text:p>` + Escape(p) + `</text:p>`)
	}
	b.WriteString(`</office:text>`)
	return b.String()
}

// ODT writes an ODT with one paragraph per text.
func ODT(paragraphs ...string) []byte { return ODFWith(MimeODT, ODFContent(ODTBody(paragraphs...))) }

// ODS writes an ODS with one cell per text in one row.
func ODS(cells ...string) []byte {
	var b strings.Builder
	b.WriteString(`<office:spreadsheet><table:table table:name="Sheet1"><table:table-row>`)
	for _, c := range cells {
		b.WriteString(`<table:table-cell office:value-type="string"><text:p>` + Escape(c) + `</text:p></table:table-cell>`)
	}
	b.WriteString(`</table:table-row></table:table></office:spreadsheet>`)
	return ODFWith(MimeODS, ODFContent(b.String()))
}

// ODP writes an ODP with one page per text.
func ODP(pages ...string) []byte {
	var b strings.Builder
	b.WriteString(`<office:presentation>`)
	for i, p := range pages {
		fmt.Fprintf(&b, `<draw:page draw:name="page%d"><draw:frame><draw:text-box><text:p>%s</text:p></draw:text-box></draw:frame></draw:page>`, i+1, Escape(p))
	}
	b.WriteString(`</office:presentation>`)
	return ODFWith(MimeODP, ODFContent(b.String()))
}

// Deflate returns data compressed with deflate, for raw entries.
func Deflate(data []byte) []byte {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	fw, err := w.CreateHeader(&zip.FileHeader{Name: "x", Method: zip.Deflate})
	must(err)
	_, err = fw.Write(data)
	must(err)
	must(w.Close())
	// The compressed body of the single entry: after the local header (30 bytes + name + extra).
	b := buf.Bytes()
	nameLen := int(binary.LittleEndian.Uint16(b[26:28]))
	extraLen := int(binary.LittleEndian.Uint16(b[28:30]))
	start := 30 + nameLen + extraLen
	r, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	must(err)
	return append([]byte(nil), b[start:start+int(r.File[0].CompressedSize64)]...) //nolint:gosec // G115: a small test body
}

// BombDeclared is a DOCX whose word/document.xml declares size bytes uncompressed (and really
// inflates to that): a zip bomb caught by the declared sizes before inflating.
func BombDeclared(size int) []byte {
	doc := DOCXDocument(Paragraphs(strings.Repeat("a", size)))
	return DOCXWith(doc)
}

// BombLyingHeader is a DOCX whose word/document.xml declares declared bytes but inflates to
// actual bytes (written with CreateRaw).
func BombLyingHeader(declared uint64, actual int) []byte {
	doc := []byte(DOCXDocument(Paragraphs(strings.Repeat("a", actual))))
	return Zip(
		Entry{Name: partContentTypes, Data: contentTypes(partDocument,
			"application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml")},
		Entry{Name: partRels, Data: rels(partDocument)},
		Entry{Name: partDocument, Raw: true, Method: zip.Deflate, Data: Deflate(doc), DeclaredSize: declared,
			CRC32: crc32.ChecksumIEEE(doc)},
	)
}

// TooManyEntries is a DOCX with n extra small entries.
func TooManyEntries(n int) []byte {
	extra := make([]Entry, 0, n)
	for i := range n {
		extra = append(extra, Entry{Name: fmt.Sprintf("customXml/item%d.xml", i), Data: []byte("<a/>")})
	}
	return DOCXWith(DOCXDocument(Paragraphs("x")), extra...)
}

// rawDocument is word/document.xml as a raw entry with the given method and flags.
func rawDocument(method, flags uint16) []byte {
	doc := []byte(DOCXDocument(Paragraphs("secret")))
	return Zip(
		Entry{Name: partContentTypes, Data: contentTypes(partDocument,
			"application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml")},
		Entry{Name: partRels, Data: rels(partDocument)},
		Entry{Name: partDocument, Raw: true, Method: method, Flags: flags, Data: doc,
			DeclaredSize: uint64(len(doc)), CRC32: crc32.ChecksumIEEE(doc)},
	)
}

// Encrypted is a DOCX whose document part carries the encryption flag.
func Encrypted() []byte { return rawDocument(zip.Store, 0x1) }

// UnknownMethod is a DOCX whose document part uses compression method 99.
func UnknownMethod() []byte { return rawDocument(99, 0) }

// DuplicatePart is a DOCX with word/document.xml twice.
func DuplicatePart() []byte {
	return DOCXWith(DOCXDocument(Paragraphs("one")), Entry{Name: partDocument, Data: []byte(DOCXDocument(Paragraphs("two")))})
}

// The XML variants of brief M3-15 design note 14. Each is a part whose root holds a reference
// that a vulnerable parser would expand; text names the element the reference sits in (w:t for
// DOCX, text:p for ODT).

// XXESystem declares an external entity at url (http:// or file://) and uses it in the text.
func XXESystem(url string) XMLVariant {
	return XMLVariant{Prolog: `<!DOCTYPE d [<!ENTITY ext SYSTEM "` + url + `">]>`, Ref: "&ext;"}
}

// XMLVariant is a prolog (after the XML declaration) and a reference used inside the text.
type XMLVariant struct {
	Prolog string
	Ref    string
	// Encoding replaces the encoding of the XML declaration when set.
	Encoding string
}

// The fixed variants.
var (
	// ParameterEntity declares a parameter entity that would load an external DTD.
	ParameterEntity = XMLVariant{Prolog: `<!DOCTYPE d [<!ENTITY % p SYSTEM "http://127.0.0.1:1/p.dtd"> %p;]>`, Ref: "x"}
	// InternalExpansion is a billion-laughs style internal entity expansion.
	InternalExpansion = XMLVariant{Prolog: `<!DOCTYPE d [<!ENTITY a "aaaaaaaaaa"><!ENTITY b "&a;&a;&a;&a;&a;&a;&a;&a;&a;&a;">]>`, Ref: "&b;"}
	// PlainDoctype is a document type declaration without entities.
	PlainDoctype = XMLVariant{Prolog: `<!DOCTYPE d>`, Ref: "x"}
	// UndefinedEntity uses an entity nobody declared.
	UndefinedEntity = XMLVariant{Ref: "&nope;"}
	// NonUTF8 declares the encoding ISO-8859-1.
	NonUTF8 = XMLVariant{Ref: "x", Encoding: "ISO-8859-1"}
)

// header returns the XML declaration and prolog of v.
func (v XMLVariant) header() string {
	enc := "UTF-8"
	if v.Encoding != "" {
		enc = v.Encoding
	}
	return `<?xml version="1.0" encoding="` + enc + `"?>` + "\n" + v.Prolog
}

// DOCX returns a DOCX whose document uses v inside a w:t.
func (v XMLVariant) DOCX() []byte {
	doc := v.header() + `<w:document xmlns:w="` + nsW + `"><w:body><w:p><w:r><w:t>before ` + v.Ref +
		` after</w:t></w:r></w:p></w:body></w:document>`
	return DOCXWith(doc)
}

// ODT returns an ODT whose content uses v inside a text:p.
func (v XMLVariant) ODT() []byte {
	content := v.header() + `<office:document-content xmlns:office="` + nsOffice + `" xmlns:text="` + nsText +
		`"><office:body><office:text><text:p>before ` + v.Ref + ` after</text:p></office:text></office:body></office:document-content>`
	return ODFWith(MimeODT, content)
}
