// SPDX-License-Identifier: Apache-2.0

package mimetype

import (
	"math"

	"github.com/gabriel-vasile/mimetype"

	"github.com/OWNER/bothy/services/server/internal/files/app"
)

// Detector detects the media type of a file from its first file.sniff_bytes for the scan pass
// (docs/spec/12-files.md 6.1, source Q14). It reads only the buffered head and never unpacks an
// archive (SEC-UP-7): mimetype.Detect inspects the bytes it is given and opens no container
// beyond them. Build it with NewDetector, which sets the read limit of the library to
// file.sniff_bytes; the zero value reads only the library's default of 4 KiB. It is safe for
// concurrent use.
type Detector struct{}

// NewDetector returns the detector and sets the read limit of the library to sniffBytes
// (file.sniff_bytes, at least 1): the library cuts every input to a process-wide limit whose
// default is 4 KiB, while Office formats need more than that (12 6.1, docs/DECISIONS.md
// D-M3-04-6). The limit is process-wide; this adapter is its only user.
func NewDetector(sniffBytes int) Detector {
	if sniffBytes > 0 {
		mimetype.SetLimit(uint32(min(sniffBytes, math.MaxUint32)))
	}
	return Detector{}
}

// Compile-time check: Detector is the outgoing port of app.
var _ app.TypeDetector = Detector{}

// Detect implements app.TypeDetector: the detected type as the library names it, parameters
// included (the pass strips them), application/octet-stream for an empty head or unknown bytes.
//
// Example:
//
//	t := mimetype.Detector{}.Detect(head) // "image/png"
func (Detector) Detect(head []byte) string {
	if len(head) == 0 {
		// The library calls nothing text/plain; no bytes say nothing about the type.
		return octetStream
	}
	return mimetype.Detect(head).String()
}

// octetStream is the type of bytes nothing matches.
const octetStream = "application/octet-stream"
