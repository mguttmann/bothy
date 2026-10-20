// SPDX-License-Identifier: Apache-2.0

package control

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
)

// DataPathPrefix is the start of every data_path of session.open (asyncapi channel hostData).
const DataPathPrefix = "/api/v1/hosts/sessions/"

// Resender is the optional part of a Sessions that reports ends it could not send before: the
// client calls Resend on every ready, after the report (M2-45, 24 6.2: the host service reports
// such an end after the next login).
type Resender interface {
	// Resend sends the pending session.ended over n.
	Resend(ctx context.Context, n Notifier)
}

// ErrDataPath is a data_path of session.open that is not a path of the channel hostData.
var ErrDataPath = errors.New("control: data_path is not a path of a data connection")

// DecodeHello decodes hello, the first text frame of a data connection, with the strict rules of
// the control connection. Errors wrap ErrFrame.
func DecodeHello(b []byte) (*Hello, error) {
	op, v, err := decodeServerFrame(b)
	if err != nil {
		return nil, err
	}
	h, ok := v.(*Hello)
	if op != opHello || !ok {
		return nil, ErrFrame
	}
	return h, nil
}

// HTTPClient returns the HTTP client of the control connection (the system roots or the stored
// CA, the proxy of the environment, no redirects); the data connections of M2-45 dial with it, so
// both connections trust exactly the same server.
func (c *Client) HTTPClient() *http.Client { return c.http }

// DataURL returns the wss URL of the data connection with path, the data_path of session.open, on
// the server of the pairing record. Errors: ErrDataPath for a path outside the channel hostData,
// so a server can never make the host service dial another host or path.
func (c *Client) DataURL(path string) (string, error) {
	if !strings.HasPrefix(path, DataPathPrefix) || strings.ContainsAny(path, "?#") || strings.Contains(path, "..") {
		return "", ErrDataPath
	}
	u, err := url.Parse(c.url)
	if err != nil {
		return "", err
	}
	base := strings.TrimSuffix(u.Path, Path)
	u.Path = base + path
	return u.String(), nil
}

// PeerLocal reports whether the server of the current control connection runs on this machine
// (24 2.3); the session manager or-s it with HD5 for step 13 of the token check.
func (c *Client) PeerLocal() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.live != nil && c.live.peerLocal
}
