// SPDX-License-Identifier: Apache-2.0

// Package extract is the text extraction of the files module (docs/spec/14-search.md 5.2 and
// 5.3, R-SU-07): pure Go adapters behind the outgoing port app.TextExtractor, with no cgo, no
// external process and no network. Registry maps the detected media type to one adapter:
//
//   - builtin-text: every text/* type except text/html and text/rtf, and application/json; UTF-8
//     with invalid bytes replaced, or UTF-16 announced by a byte order mark.
//   - builtin-html: text/html through the tokenizer of golang.org/x/net/html; text nodes only,
//     script and style dropped, block elements separate words.
//   - builtin-ooxml: DOCX, XLSX and PPTX through archive/zip and encoding/xml (word/document.xml,
//     xl/sharedStrings.xml, ppt/slides/slide<N>.xml).
//   - builtin-odf: ODT, ODS and ODP (content.xml).
//
// Archives are opened only in memory, within search.extract.max_uncompressed_bytes and
// search.extract.max_archive_entries; nested archives and embedded objects are never opened and
// nothing is written to the file system. XML is read by encoding/xml in strict mode without
// entities beyond the predefined ones; a document type declaration (the carrier of every external
// entity) fails the file with xml_doctype, and a run guard bounds every token by
// search.extract.max_text_bytes.
//
// Each adapter has a version (<adapter>/<n>, the value of file_text_extracts.extractor). The
// golden files in testdata/golden hold the SHA-256 of each adapter's output over the fixtures of
// extracttest; TestExtractorGolden fails when an output changes without a version rise, so the
// job files.extract_text_refresh re-extracts what an older version produced.
//
// Example:
//
//	reg, err := extract.New(extract.Config{MaxTextBytes: 1 << 20, MaxUncompressedBytes: 64 << 20, MaxArchiveEntries: 10000})
//	res, err := reg.Extract(ctx, body, "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
package extract
