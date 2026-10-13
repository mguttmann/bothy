// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"errors"
	"net/url"
	"path"
	"strings"
	"unicode"
)

// SinkScheme is the one scheme of audit.checkpoint_sink this version supports; an object storage
// with retention lock comes with PLAN M7-22.
const SinkScheme = "file"

// errSink is the error of every value ParseSink refuses; it names the supported form and never
// the value.
var errSink = errors.New("audit.checkpoint_sink must be empty or file://<absolute path>; the supported scheme is file://")

// ParseSink reads the value of audit.checkpoint_sink (19 section 16): empty means no sink and
// returns "", otherwise exactly "file://" and an absolute path without host, user, query,
// fragment, percent escapes or control characters and with a clean path (no "..", no trailing
// slash). It returns the directory.
func ParseSink(v string) (string, error) {
	if v == "" {
		return "", nil
	}
	// The exact prefix and no escapes: url.Parse lower-cases the scheme, takes "file:/x" and
	// decodes %00, all forms the documented value does not have.
	if !strings.HasPrefix(v, SinkScheme+"://") || strings.ContainsAny(v, "%\x00") || strings.ContainsFunc(v, unicode.IsControl) {
		return "", errSink
	}
	u, err := url.Parse(v)
	if err != nil || u.Scheme != SinkScheme || u.Opaque != "" || u.Host != "" || u.User != nil ||
		u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.RawFragment != "" {
		return "", errSink
	}
	p := u.Path
	if p == "" || !path.IsAbs(p) || path.Clean(p) != p || p == "/" {
		return "", errSink
	}
	return p, nil
}
