// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// headerHSTS is the header of SEC-TLS-3.
const headerHSTS = "Strict-Transport-Security"

// schemeHTTPS is the scheme of server.public_url that switches HSTS on and that TLS mode needs.
const schemeHTTPS = "https"

// SecurityHeaders are the headers of SEC-HDR-1 that every response of both listeners carries:
// X-Content-Type-Options: nosniff, Referrer-Policy: no-referrer, Strict-Transport-Security while
// server.public_url uses https (SEC-TLS-3), and no Server header. The zero value sets everything
// but HSTS. Build it with SecurityHeadersFor; it is immutable and safe for concurrent use.
type SecurityHeaders struct {
	hsts string
}

// SecurityHeadersFor returns the SEC-HDR-1 headers for server.public_url and http.hsts_max_age.
// With the scheme https HSTS is "max-age=<seconds>" without includeSubDomains (other subdomains
// belong to the operator) and without preload (not reversible); max-age=0 is valid and tells
// browsers to forget the rule. With any other scheme or a nil URL (loopback development, 16
// "6.1 Umgebungen") there is no HSTS. hstsMaxAge is cut to whole seconds and a negative value
// counts as 0; the configuration schema already refuses both (hsts_whole_seconds).
//
// Example:
//
//	sec := httpserver.SecurityHeadersFor(publicURL, 365*24*time.Hour)
//	sec.Apply(w.Header()) // Strict-Transport-Security: max-age=31536000
func SecurityHeadersFor(publicURL *url.URL, hstsMaxAge time.Duration) SecurityHeaders {
	if publicURL == nil || publicURL.Scheme != schemeHTTPS {
		return SecurityHeaders{}
	}
	seconds := max(int64(hstsMaxAge/time.Second), 0)
	return SecurityHeaders{hsts: "max-age=" + strconv.FormatInt(seconds, 10)}
}

// Apply sets the headers on h, replacing values a handler set, and removes Server.
func (s SecurityHeaders) Apply(h http.Header) {
	h.Set(headerNoSniff, valueNoSniff)
	h.Set(headerReferrer, valueNoReferrer)
	if s.hsts != "" {
		h.Set(headerHSTS, s.hsts)
	} else {
		h.Del(headerHSTS)
	}
	h.Del(headerServer)
}

// secureWriter applies its SecurityHeaders once more right before the response header goes out,
// so no inner component can drop or weaken them (SEC-HDR-1).
type secureWriter struct {
	http.ResponseWriter
	sec  SecurityHeaders
	sent bool
}

// secure applies the headers before the first header block, also before 1xx responses.
func (w *secureWriter) secure() {
	if !w.sent {
		w.sec.Apply(w.Header())
	}
}

// WriteHeader applies the headers and passes the call on; a final status ends the header block.
func (w *secureWriter) WriteHeader(code int) {
	w.secure()
	if code >= http.StatusOK || code == http.StatusSwitchingProtocols {
		w.sent = true
	}
	w.ResponseWriter.WriteHeader(code)
}

// Write applies the headers when nothing was written yet and passes the call on.
func (w *secureWriter) Write(b []byte) (int, error) {
	w.secure()
	w.sent = true
	return w.ResponseWriter.Write(b)
}

// Flush applies the headers and flushes the writer below when it can.
func (w *secureWriter) Flush() {
	w.secure()
	w.sent = true
	_ = http.NewResponseController(w.ResponseWriter).Flush()
}

// Unwrap returns the writer below for http.ResponseController.
func (w *secureWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// InternalHandler returns the handler of the internal listener for mux (Internal and the routes
// later tickets add): every response carries sec (SEC-HDR-1) and the API headers Cache-Control:
// no-store and Cross-Origin-Resource-Policy: same-origin (SEC-HDR-2; health answers JSON), and a
// request no pattern matches gets 404 not_found or 405 method_not_allowed with Allow as a problem
// (06 "Errors") instead of the plain text of net/http. It reads no identity or forwarding header
// and has no peer filter (R-SE-09).
func InternalHandler(mux *http.ServeMux, sec SecurityHeaders) http.Handler {
	dispatch := dispatchMux(mux)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		sec.Apply(h)
		h.Set(headerCacheControl, valueNoStore)
		h.Set(headerCORP, valueSameOrigin)
		dispatch.ServeHTTP(&secureWriter{ResponseWriter: w, sec: sec}, r)
	})
}

// dispatchMux sends requests with a matching pattern to mux; for the others the mux's own handler
// runs against a probe writer to learn 404 or 405 and Allow, and the answer is a problem instead of
// the plain text of net/http. A path that is not in clean form gets 404, because net/http would
// answer a redirect to the clean path itself, outside the chain.
func dispatchMux(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !cleanPath(r.URL.Path) {
			writeProblem(w, r, errs.New(errs.CodeNotFound, ""))
			return
		}
		h, pattern := mux.Handler(r)
		if pattern != "" {
			mux.ServeHTTP(w, r)
			return
		}
		probe := &probeWriter{header: http.Header{}}
		h.ServeHTTP(probe, r)
		if probe.status == http.StatusMethodNotAllowed {
			e := errs.New(errs.CodeMethodNotAllowed, "")
			if allow := probe.header.Get(headerAllow); allow != "" {
				e = e.WithHeader(headerAllow, allow)
			}
			writeProblem(w, r, e)
			return
		}
		writeProblem(w, r, errs.New(errs.CodeNotFound, ""))
	})
}
