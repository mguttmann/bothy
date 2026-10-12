// SPDX-License-Identifier: Apache-2.0

// Test cases for the rule bothy-go-egress-only (run by `make semgrep`).
package fixtures

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/egress"

	nethttp "net/http"
)

func defaults(u *url.URL) {
	// ruleid: bothy-go-egress-only
	_ = http.DefaultClient
	// ruleid: bothy-go-egress-only
	_, _ = http.DefaultClient.Do(nil)
	// ruleid: bothy-go-egress-only
	_ = http.DefaultTransport
	// ruleid: bothy-go-egress-only
	_, _ = http.Get("https://api.example.com/")
	// ruleid: bothy-go-egress-only
	_, _ = http.Head("https://api.example.com/")
	// ruleid: bothy-go-egress-only
	_, _ = http.Post("https://api.example.com/", "text/plain", nil)
	// ruleid: bothy-go-egress-only
	_, _ = http.PostForm("https://api.example.com/", url.Values{})
	// ruleid: bothy-go-egress-only
	_ = httputil.NewSingleHostReverseProxy(u)
	// ruleid: bothy-go-egress-only
	_ = &httputil.ReverseProxy{}
}

func ownClients() {
	// ruleid: bothy-go-egress-only
	_ = http.Client{}
	// ruleid: bothy-go-egress-only
	_ = &http.Client{Timeout: time.Second}
	// ruleid: bothy-go-egress-only
	_ = new(http.Client)
	// ruleid: bothy-go-egress-only
	var c http.Client
	_ = c
	// ruleid: bothy-go-egress-only
	_ = http.Transport{}
	// ruleid: bothy-go-egress-only
	_ = &http.Transport{Proxy: nil}
	// ruleid: bothy-go-egress-only
	_ = new(http.Transport)
	// ruleid: bothy-go-egress-only
	var tr http.Transport
	_ = &tr
}

func ownDialers() {
	// ruleid: bothy-go-egress-only
	_, _ = net.Dial("tcp", "192.0.2.10:443")
	// ruleid: bothy-go-egress-only
	_, _ = net.DialTimeout("tcp", "192.0.2.10:443", time.Second)
	// ruleid: bothy-go-egress-only
	_, _ = net.DialTCP("tcp", nil, nil)
	// ruleid: bothy-go-egress-only
	_, _ = net.DialUDP("udp", nil, nil)
	// ruleid: bothy-go-egress-only
	_, _ = net.DialIP("ip4:icmp", nil, nil)
	// ruleid: bothy-go-egress-only
	_ = net.Dialer{}
	// ruleid: bothy-go-egress-only
	_ = &net.Dialer{Timeout: time.Second}
	// ruleid: bothy-go-egress-only
	_ = new(net.Dialer)
	// ruleid: bothy-go-egress-only
	var d net.Dialer
	_ = d
	// ruleid: bothy-go-egress-only
	_, _ = tls.Dial("tcp", "192.0.2.10:443", nil)
	// ruleid: bothy-go-egress-only
	_, _ = tls.DialWithDialer(nil, "tcp", "192.0.2.10:443", nil)
}

func functionValues() {
	// ruleid: bothy-go-egress-only
	get := http.Get
	_ = get
	// ruleid: bothy-go-egress-only
	dial := net.Dial
	_ = dial
}

func tlsDialers() {
	// ruleid: bothy-go-egress-only
	_ = tls.Dialer{}
	// ruleid: bothy-go-egress-only
	_ = &tls.Dialer{NetDialer: nil}
	// ruleid: bothy-go-egress-only
	_ = new(tls.Dialer)
	// ruleid: bothy-go-egress-only
	var td tls.Dialer
	_ = td
}

// ruleid: bothy-go-egress-only
type fetcher struct {
	name   string
	client http.Client
}

// The finding of an embedded value is on the field line.
type embedded struct {
	// ruleid: bothy-go-egress-only
	http.Client
}

// ruleid: bothy-go-egress-only
type dialers struct {
	tcp net.Dialer
}

// ruleid: bothy-go-egress-only
type tlsDialerField struct {
	dialer tls.Dialer
}

// ruleid: bothy-go-egress-only
type transportField struct {
	rt http.Transport
}

// ruleid: bothy-go-egress-only
type clientAlias = http.Client

// ruleid: bothy-go-egress-only
var anonymous struct {
	client http.Client
}

// ok: bothy-go-egress-only
type consumer struct {
	client *http.Client
	rt     http.RoundTripper
}

// ok: bothy-go-egress-only
type embeddedPointer struct {
	*http.Client
}

// ruleid: bothy-go-egress-only
type definedClient http.Client

// ruleid: bothy-go-egress-only
type collectionFields struct {
	clients [2]http.Client
	dialers map[string]net.Dialer
	tls     []tls.Dialer
}

// ruleid: bothy-go-egress-only
type nestedValue struct {
	inner struct {
		client http.Client
	}
}

// ruleid: bothy-go-egress-only
var clientArray [2]http.Client

// ruleid: bothy-go-egress-only
var clientSlice []http.Client

// ok: bothy-go-egress-only
var clientPointer *http.Client

func collections(hc *http.Client) {
	// ruleid: bothy-go-egress-only
	_ = []http.Client{{}}
	// ruleid: bothy-go-egress-only
	_ = [2]net.Dialer{}
	// ruleid: bothy-go-egress-only
	_ = map[string]http.Transport{}
	// ruleid: bothy-go-egress-only
	_ = []tls.Dialer{{}}
	// ruleid: bothy-go-egress-only
	_ = [1]tls.Dialer{}
	// ruleid: bothy-go-egress-only
	_ = map[string]tls.Dialer{}
	// ruleid: bothy-go-egress-only
	_ = struct{ client http.Client }{}
	// ok: bothy-go-egress-only
	_ = []*http.Client{hc}
	// ruleid: bothy-go-egress-only
	hc.Transport = nil
}

func renamedImport() {
	// ruleid: bothy-go-egress-only
	_, _ = nethttp.Get("https://api.example.com/")
	// ruleid: bothy-go-egress-only
	_ = &nethttp.Client{}
}

func allowed(ctx context.Context, cfg egress.Config, opts egress.Options, deps egress.Deps) error {
	// ok: bothy-go-egress-only
	client, err := egress.New(cfg, opts, deps)
	if err != nil {
		return err
	}
	// ok: bothy-go-egress-only
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.example.com/", nil)
	if err != nil {
		return err
	}
	// ok: bothy-go-egress-only
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	// ok: bothy-go-egress-only
	_ = egress.NewLoopback(time.Second)
	// ok: bothy-go-egress-only
	_ = client.HTTPClient()
	// ok: bothy-go-egress-only
	_ = &http.Server{ReadHeaderTimeout: time.Second}
	// ok: bothy-go-egress-only
	_, _ = net.Listen("tcp", "127.0.0.1:0")
	// ok: bothy-go-egress-only
	_ = &tls.Config{MinVersion: tls.VersionTLS12}
	return resp.Body.Close()
}
