// SPDX-License-Identifier: Apache-2.0

package files

import (
	"io"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The variants of a download (12 7.3; openapi FileTicketCreate.variant).
const (
	// VariantOriginal is the file itself (GET /files/{id}/content).
	VariantOriginal = "original"
	// VariantSmall, VariantMedium, VariantLarge and VariantPoster are the thumbnails.
	VariantSmall  = "small"
	VariantMedium = "medium"
	VariantLarge  = "large"
	VariantPoster = "poster"
)

// OpenRequest is one download (downloadFile with VariantOriginal, downloadThumbnail with a
// thumbnail variant). Range and If-Range are the raw request headers; the use case decides them.
type OpenRequest struct {
	// FileID is the file of the path.
	FileID ids.ID
	// Variant is VariantOriginal or a thumbnail variant of the route.
	Variant string
	// Ticket is the query parameter ticket; "" when absent. It is a bearer secret: never log it.
	Ticket string
	// Range and IfRange are the request headers; "" when absent.
	Range, IfRange string
}

// Delivery is an opened download: the status, the headers of 12 9.3 and the body. The adapter
// writes the headers and streams Body; it decides nothing.
type Delivery struct {
	// Status is 200 (the whole variant) or 206 (one range).
	Status int
	// Size is the size of the whole variant in bytes.
	Size int64
	// First and Last are the served bytes, inclusive; the whole variant with 200.
	First, Last int64
	// ContentType and Disposition are the Content-Type and Content-Disposition values.
	ContentType, Disposition string
	// ETag is "<sha256 hex>" in quotes, ReprDigest sha-256=:<base64>: of the whole variant.
	ETag, ReprDigest string
	// CacheControl is private, max-age=<client.cache_max_age>, immutable with normal
	// authentication and private, no-store with a ticket.
	CacheControl string
	// Ticket reports that a ticket authenticated the request.
	Ticket bool
	// Body streams the served bytes; the caller closes it.
	Body io.ReadCloser
}

// Length returns the number of served bytes (Content-Length).
func (d Delivery) Length() int64 { return d.Last - d.First + 1 }

// IssuedTicket is the answer of createFileTicket (schema FileTicket).
type IssuedTicket struct {
	// URL is the relative reference of the download with the ticket in the query parameter
	// ticket. It is a bearer secret until ExpiresAt: never log it.
	URL string
	// ExpiresAt is the end of validity, now plus file.ticket_ttl in whole seconds.
	ExpiresAt time.Time
}

// TranscriptView is the answer of getTranscript (schema Transcript): Text, Language and Source
// are set only with Status done.
type TranscriptView struct {
	FileID                         ids.ID
	Status, Text, Language, Source string
}
