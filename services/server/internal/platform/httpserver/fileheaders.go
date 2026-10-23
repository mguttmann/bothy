// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"net/http"
	"strings"
)

// The headers of a file response (SEC-HDR-4, docs/spec/12-files.md 9.3).
const (
	headerContentDisposition = "Content-Disposition"
	// valueFileCSP keeps a file that a browser opens from running anything (SEC-HDR-4).
	valueFileCSP = "default-src 'none'; sandbox"
)

// SetFileHeaders sets the headers of SEC-HDR-4 that every response with the bytes of a stored
// file or variant carries: Content-Type, Content-Disposition with filename and filename* (RFC
// 6266, RFC 8187), inline only for the inline list of 12 9.3 (inline true) and attachment
// otherwise, Content-Security-Policy: default-src 'none'; sandbox and X-Content-Type-Options:
// nosniff. Cache-Control and Cross-Origin-Resource-Policy stay with the chain or the caller (12
// 9.3). It is the one writer of these headers; the download routes of M3-05 use it as well.
//
// Example:
//
//	httpserver.SetFileHeaders(w.Header(), "image/png", "logo.png", true)
func SetFileHeaders(h http.Header, contentType, filename string, inline bool) {
	disposition := "attachment"
	if inline {
		disposition = "inline"
	}
	h.Set(headerContentType, contentType)
	h.Set(headerContentDisposition, disposition+`; filename="`+asciiFilename(filename)+`"; filename*=UTF-8''`+extValue(filename))
	h.Set(headerCSP, valueFileCSP)
	h.Set(headerNoSniff, valueNoSniff)
}

// asciiFilename is the quoted-string fallback of filename: every byte outside printable ASCII, the
// quote and the backslash become "_", so the value needs no escaping and never ends the header.
func asciiFilename(name string) string {
	var b strings.Builder
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c < 0x20 || c > 0x7e || c == '"' || c == '\\' {
			b.WriteByte('_')
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

// extValue percent-encodes name as the value-chars of RFC 8187: attr-char stays, every other byte
// of the UTF-8 form becomes %XX.
func extValue(name string) string {
	const hex = "0123456789ABCDEF"
	var b strings.Builder
	for i := 0; i < len(name); i++ {
		c := name[i]
		if attrChar(c) {
			b.WriteByte(c)
			continue
		}
		b.WriteByte('%')
		b.WriteByte(hex[c>>4])
		b.WriteByte(hex[c&0x0f])
	}
	return b.String()
}

// attrChar reports the attr-char of RFC 8187: ALPHA, DIGIT and !#$&+-.^_`|~.
func attrChar(c byte) bool {
	switch {
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		return true
	}
	return strings.IndexByte("!#$&+-.^_`|~", c) >= 0
}
