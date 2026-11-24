// SPDX-License-Identifier: Apache-2.0

package mimetype

import (
	"sync"

	"github.com/gabriel-vasile/mimetype"

	"github.com/OWNER/bothy/services/server/internal/files/app"
)

// Detector detects the media type of a file from its first file.sniff_bytes for the scan pass
// (docs/spec/12-files.md 6.1, source Q14). It reads only the buffered head and never unpacks an
// archive (SEC-UP-7): mimetype.Detect inspects the bytes it is given and opens no container
// beyond them. NewDetector bounds the head to file.sniff_bytes; the zero value reads the whole
// head it is given. It is safe for concurrent use.
type Detector struct {
	limit int
}

// noLibraryLimit lifts the process-wide read limit of the library (default 4 KiB, too small for
// Office formats, 12 6.1, docs/DECISIONS.md D-M3-04-6) once, to "the whole input"; each Detector
// bounds its head itself, so no detector changes what another one reads.
var noLibraryLimit sync.Once

// NewDetector returns the detector that reads at most sniffBytes (file.sniff_bytes) of a head; a
// value below 1 reads the whole head.
func NewDetector(sniffBytes int) Detector {
	return Detector{limit: max(sniffBytes, 0)}
}

// Compile-time check: Detector is the outgoing port of app.
var _ app.TypeDetector = Detector{}

// Detect implements app.TypeDetector: the detected type as the library names it, parameters
// included (the pass strips them), application/octet-stream for an empty head or unknown bytes.
//
// Example:
//
//	t := mimetype.Detector{}.Detect(head) // "image/png"
func (d Detector) Detect(head []byte) string {
	if d.limit > 0 && len(head) > d.limit {
		head = head[:d.limit]
	}
	noLibraryLimit.Do(func() { mimetype.SetLimit(0) })
	if len(head) == 0 {
		// The library calls nothing text/plain; no bytes say nothing about the type.
		return octetStream
	}
	return mimetype.Detect(head).String()
}

// octetStream is the type of bytes nothing matches.
const octetStream = "application/octet-stream"
