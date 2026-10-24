// SPDX-License-Identifier: Apache-2.0

package domain

import "unicode/utf8"

// The values of file_text_extracts.state (docs/spec/03-data-model.md 4.7, 14 5.1).
const (
	// ExtractPending: the row waits for the job files.extract_text.
	ExtractPending = "pending"
	// ExtractDone: text_content holds the extracted text (possibly empty).
	ExtractDone = "done"
	// ExtractFailed: error holds a code of ExtractCode, text_content is NULL.
	ExtractFailed = "failed"
	// ExtractUnsupported: no extractor supports the detected type at run time.
	ExtractUnsupported = "unsupported"
)

// ExtractCode is the closed set of codes file_text_extracts.error holds for a failed extraction
// (brief M3-15 design note 4, docs/decisions.d/M3-15.md): a code, never content (03 4.7).
type ExtractCode string

// The codes of a failed extraction.
const (
	// CodeInputTooLarge: the object is larger than search.extract.max_input_bytes.
	CodeInputTooLarge ExtractCode = "input_too_large"
	// CodeArchiveInvalid: the container is no readable zip archive, has an ambiguous, encrypted or
	// unknown-method part, lacks a required part, or a part inflates beyond its declared size.
	CodeArchiveInvalid ExtractCode = "archive_invalid"
	// CodeArchiveEntriesExceeded: the central directory lists more than
	// search.extract.max_archive_entries entries.
	CodeArchiveEntriesExceeded ExtractCode = "archive_entries_exceeded"
	// CodeArchiveSizeExceeded: the parts read would inflate beyond
	// search.extract.max_uncompressed_bytes.
	CodeArchiveSizeExceeded ExtractCode = "archive_size_exceeded"
	// CodeXMLDoctype: an XML part carries a document type declaration (the carrier of every
	// external entity).
	CodeXMLDoctype ExtractCode = "xml_doctype"
	// CodeXMLInvalid: an XML part is not well-formed, uses an undefined entity or declares an
	// encoding other than UTF-8.
	CodeXMLInvalid ExtractCode = "xml_invalid"
	// CodeTimeout: the extraction passed search.extract.timeout.
	CodeTimeout ExtractCode = "timeout"
	// CodeObjectMissing: the object of a deliverable file is not in the object store.
	CodeObjectMissing ExtractCode = "object_missing"
)

// ExtractError is a content problem of one extraction: the file ends failed with Code. Its text
// names only the code, never a part name or content (SEC-LOG-4).
type ExtractError struct {
	// Code is the value written to file_text_extracts.error.
	Code ExtractCode
}

// Error implements error with the code only.
func (e *ExtractError) Error() string { return "files: text extraction failed: " + string(e.Code) }

// NewExtractError returns the error of code.
//
// Example:
//
//	return domain.NewExtractError(domain.CodeArchiveInvalid)
func NewExtractError(code ExtractCode) *ExtractError { return &ExtractError{Code: code} }

// ExtractApplies reports whether a file gets a row in file_text_extracts and the job
// files.extract_text (14 5.1 item 1, brief M3-15 design note 2): exactly an attachment that is
// deliverable (IsDeliverable) and whose detected type an extractor supports. Avatars, logos and
// data exports are never searchable (14 2), so their text would be a copy nobody can reach.
//
// Example:
//
//	domain.ExtractApplies("attachment", true, true) // true
func ExtractApplies(purpose string, deliverable, supported bool) bool {
	return Purpose(purpose) == PurposeAttachment && deliverable && supported
}

// RefreshRow is what the walk of files.extract_text_refresh knows of one deliverable attachment
// (brief M3-15 design note 9): its extract row, if any, and its size.
type RefreshRow struct {
	// Exists reports that the file has a row in file_text_extracts.
	Exists bool
	// State, Extractor and Error are the columns of the row; Extractor and Error are "" for NULL.
	State, Extractor, Error string
	// SizeBytes is files.size_bytes.
	SizeBytes int64
}

// RefreshAction is the decision of RefreshDecision for one file.
type RefreshAction uint8

// The decisions of the walk.
const (
	// RefreshNothing leaves the file alone.
	RefreshNothing RefreshAction = iota
	// RefreshCreate inserts the row as the trigger does and enqueues files.extract_text.
	RefreshCreate
	// RefreshEnqueue enqueues files.extract_text again for a pending row (a job River discarded).
	RefreshEnqueue
	// RefreshReset sets the row pending again (compare-and-set) and enqueues files.extract_text.
	RefreshReset
)

// RefreshDecision decides one file of the walk (14 5.1 item 4, the table of brief M3-15 design
// note 9): current is the version the extractor reports for the file's type, supported whether it
// supports it, maxInputBytes is search.extract.max_input_bytes. A row of an older or no extractor
// version is reset; a row that failed only for its size is reset when the limit now admits it; a
// pending row is enqueued again; a missing row of a supported type is created.
//
// Example:
//
//	domain.RefreshDecision(domain.RefreshRow{Exists: true, State: "done", Extractor: "builtin-ooxml/1"},
//		"builtin-ooxml/2", true, 1<<20) // RefreshReset
func RefreshDecision(row RefreshRow, current string, supported bool, maxInputBytes int64) RefreshAction {
	if !row.Exists {
		if supported {
			return RefreshCreate
		}
		return RefreshNothing
	}
	switch row.State {
	case ExtractPending:
		return RefreshEnqueue
	case ExtractDone, ExtractFailed, ExtractUnsupported:
		if !supported {
			return RefreshNothing
		}
		if row.Extractor != current {
			return RefreshReset
		}
		if row.State == ExtractFailed && row.Error == string(CodeInputTooLarge) && row.SizeBytes <= maxInputBytes {
			return RefreshReset
		}
	}
	return RefreshNothing
}

// HalveText returns the first half of the bytes of s, cut back to a rune boundary, so a text the
// search vector cannot hold is written again shorter (brief M3-15 design note 8). Repeated calls
// reach "" for every input; "" stays "".
//
// Example:
//
//	domain.HalveText("abcd") // "ab"
func HalveText(s string) string {
	n := len(s) / 2
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}
