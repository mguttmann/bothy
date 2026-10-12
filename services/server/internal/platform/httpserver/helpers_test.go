// SPDX-License-Identifier: Apache-2.0

package httpserver_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"

	"github.com/OWNER/bothy/services/server/internal/kernel/reqmeta"
)

// jsonUnmarshal decodes data into v.
func jsonUnmarshal(data []byte, v any) error { return json.Unmarshal(data, v) }

// reqmetaClientIP returns the client IP the chain stored for r as text.
func reqmetaClientIP(r *http.Request) (string, bool) {
	ip, ok := reqmeta.ClientIP(r.Context())
	return ip.String(), ok
}

// mustDecode decodes a base64url cursor without padding.
func mustDecode(t interface{ Fatal(...any) }, s string) []byte {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// encode encodes bytes as base64url without padding.
func encode(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

// decodeNumbers decodes data with json.Number, as the handlers' decoders treat numbers as text.
func decodeNumbers(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var v any
	return dec.Decode(&v)
}
