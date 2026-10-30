// SPDX-License-Identifier: Apache-2.0

//go:build sdkserver

package sdkserver_test

// The app ops of sdkserver (brief M2-49 Design note 19): the TLS front of an app outside the
// server (start member app_front, op app_backend), a forum of the world (create_forum) and any
// operation of the public API as a person of the world (call). The live test of
// plugins/github-app drives them; README.md documents them.

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/OWNER/bothy/services/server/api/routes"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
	"github.com/OWNER/bothy/services/server/test/apps/fakeapp"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// frontCertTTL is the validity of the front's certificate; a run never lasts that long.
const frontCertTTL = 24 * time.Hour

// hostName is a DNS name of letters, digits, '-' and '.', the form of an egress rule host.
var hostName = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$`)

// pathParam is a parameter of a path template of the route table.
var pathParam = regexp.MustCompile(`\{([a-z_]+)\}`)

// appFrontRequest is the member app_front of start.
type appFrontRequest struct {
	Host string `json:"host"`
}

// appFront is the TLS front of an app: it listens on 127.0.0.1 with a certificate for its host
// from a certificate authority created in memory and forwards each accepted connection, TLS
// terminated, as plain TCP to the backend port the app registered with app_backend; without one
// it closes the connection. This is the layout of a TLS proxy in front of an app (brief M2-49
// Design note 4) in small.
type appFront struct {
	host string
	ln   net.Listener
	pool *x509.CertPool

	mu      sync.Mutex
	backend int
	conns   map[net.Conn]struct{}
	closed  bool
	wg      sync.WaitGroup
}

// newAppFront starts a front for host.
func newAppFront(host string) (*appFront, error) {
	cert, pool, err := frontCertificate(host)
	if err != nil {
		return nil, err
	}
	raw, err := new(net.ListenConfig).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	f := &appFront{host: host, pool: pool, conns: map[net.Conn]struct{}{},
		ln: tls.NewListener(raw, &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12})}
	f.wg.Add(1)
	go f.serve()
	return f, nil
}

// Port is the port of the front on 127.0.0.1.
func (f *appFront) Port() int { return f.ln.Addr().(*net.TCPAddr).Port }

// HostPort is host:port, the form of the webhook.allowed_hosts entry.
func (f *appFront) HostPort() string { return f.host + ":" + strconv.Itoa(f.Port()) }

// SetBackend makes the front forward to 127.0.0.1:port.
func (f *appFront) SetBackend(port int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.backend = port
}

// egressDeps are the resolver and roots of the egress clients of the apps module: the host
// resolves to 127.0.0.1, and only the front's authority is trusted.
func (f *appFront) egressDeps() egress.Deps {
	return egress.Deps{Resolver: fakeapp.NewResolver(f.host), RootCAs: f.pool}
}

// Close stops accepting, ends the open connections and waits for the forwarders.
func (f *appFront) Close() error {
	f.mu.Lock()
	f.closed = true
	for c := range f.conns {
		_ = c.Close()
	}
	f.mu.Unlock()
	err := f.ln.Close()
	f.wg.Wait()
	return err
}

// serve accepts until the listener closes.
func (f *appFront) serve() {
	defer f.wg.Done()
	for {
		c, err := f.ln.Accept()
		if err != nil {
			return
		}
		f.wg.Add(1)
		go f.forward(c)
	}
}

// track registers c as open; false when the front is closed.
func (f *appFront) track(c net.Conn) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return false
	}
	f.conns[c] = struct{}{}
	return true
}

// untrack forgets c.
func (f *appFront) untrack(c net.Conn) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.conns, c)
}

// forward pipes the TLS connection c to the backend in both directions until one side ends.
func (f *appFront) forward(c net.Conn) {
	defer f.wg.Done()
	defer func() { _ = c.Close() }()
	f.mu.Lock()
	port := f.backend
	f.mu.Unlock()
	if port == 0 || !f.track(c) {
		return
	}
	defer f.untrack(c)
	b, err := new(net.Dialer).DialContext(context.Background(), "tcp", "127.0.0.1:"+strconv.Itoa(port))
	if err != nil {
		return
	}
	if !f.track(b) {
		_ = b.Close()
		return
	}
	defer f.untrack(b)
	defer func() { _ = b.Close() }()
	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(b, c); closeWrite(b); done <- struct{}{} }()
	go func() { _, _ = io.Copy(c, b); closeWrite(c); done <- struct{}{} }()
	<-done
	<-done
}

// closeWrite half-closes c when it supports it, so the other side sees the end of the request.
func closeWrite(c net.Conn) {
	switch v := c.(type) {
	case *net.TCPConn:
		_ = v.CloseWrite()
	case *tls.Conn:
		_ = v.CloseWrite()
	}
}

// frontCertificate creates a certificate authority in memory and a leaf for host signed by it;
// the pool trusts only that authority.
func frontCertificate(host string) (tls.Certificate, *x509.CertPool, error) {
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	now := time.Now()
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "sdkserver app front authority"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(frontCertTTL), IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign}
	caDER, err := x509.CreateCertificate(rand.Reader, ca, ca, &caKey.PublicKey, caKey)
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	leaf := &x509.Certificate{SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: host}, DNSNames: []string{host},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(frontCertTTL), KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, leaf, caCert, &key.PublicKey, caKey)
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	pool := x509.NewCertPool()
	pool.AddCert(caCert)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, pool, nil
}

// openFront starts the front of app_front before the server starts and adds its host and port to
// webhook.allowed_hosts and, with allow_private, to egress.allowed_hosts of the configuration.
func (p *program) openFront(req *appFrontRequest) error {
	if req == nil {
		return nil
	}
	host := strings.ToLower(req.Host)
	if !hostName.MatchString(host) {
		return errors.New("app_front.host must be a DNS name")
	}
	f, err := newAppFront(host)
	if err != nil {
		return err
	}
	p.front = f
	p.t.Cleanup(func() { _ = f.Close() })
	appendList(p.cfg, "webhook.allowed_hosts", f.HostPort())
	appendList(p.cfg, "egress.allowed_hosts", f.HostPort()+";allow_private")
	return nil
}

// appendList appends item to the list value of key (comma separated, configuration.md 2).
func appendList(cfg map[string]string, key, item string) {
	if v := strings.TrimSpace(cfg[key]); v != "" {
		cfg[key] = v + "," + item
		return
	}
	cfg[key] = item
}

// appBackend registers the port of the app behind the front.
func (p *program) appBackend(req request) (answer, error) {
	if p.front == nil {
		return nil, errors.New("start had no app_front")
	}
	if req.Port <= 0 || req.Port > 65535 {
		return nil, errors.New("port must be a TCP port")
	}
	p.front.SetBackend(req.Port)
	return answer{"op": "app_backend"}, nil
}

// createForum lets the owner create a forum with name in the space Support (createChannel).
func (p *program) createForum(req request) (answer, error) {
	if req.Name == "" {
		return nil, errors.New("name is required")
	}
	resp, err := harness.DoWithStepUp(p.gen.Context(), p.owner, harness.Request{Method: http.MethodPost,
		Path: "/spaces/" + p.support + "/channels", Body: map[string]any{"kind": "forum", "name": req.Name}})
	if err != nil {
		return nil, err
	}
	if resp.Status != http.StatusCreated {
		return nil, fmt.Errorf("createChannel answered %d", resp.Status)
	}
	var ch struct {
		ID      string `json:"id"`
		Channel struct {
			ID string `json:"id"`
		} `json:"channel"`
	}
	if err := json.Unmarshal(resp.Body, &ch); err != nil {
		return nil, err
	}
	id := ch.ID
	if id == "" {
		id = ch.Channel.ID
	}
	if id == "" {
		return nil, errors.New("createChannel answered without an id")
	}
	return answer{"op": "create_forum", "channel_id": id}, nil
}

// callResult is the content of the answer file of call.
type callResult struct {
	Status int             `json:"status"`
	Body   json.RawMessage `json:"body,omitempty"`
}

// call performs operation_id through the real HTTP chain as the owner (with a fresh step-up
// first) or Alex and writes status and body into a new file of mode 0600 in the dir of start (a
// raw_body, base64, is sent as it is with the header entries, for uploadPart); the
// answer names status and file only, so no token is ever printed.
func (p *program) call(req request) (answer, error) {
	var c *harness.Client
	switch req.As {
	case "owner":
		c = p.owner
	case "alex":
		c = p.alex
	default:
		return nil, fmt.Errorf("as must be owner or alex, not %q", req.As)
	}
	op, ok := routes.ByID(req.OperationID)
	if !ok {
		return nil, fmt.Errorf("unknown operation %q", req.OperationID)
	}
	path, err := fillPath(strings.TrimPrefix(op.Path, routes.BaseURL), req.PathParams)
	if err != nil {
		return nil, err
	}
	if len(req.Query) > 0 {
		q := url.Values{}
		for k, v := range req.Query {
			q.Set(k, v)
		}
		path += "?" + q.Encode()
	}
	ctx := p.gen.Context()
	if c == p.owner {
		if err := c.StepUp(ctx); err != nil {
			return nil, err
		}
	}
	r := harness.Request{Method: op.Method, Path: path, Header: http.Header{}}
	for k, v := range req.Header {
		r.Header.Set(k, v)
	}
	switch {
	case req.RawBody != "":
		raw, err := base64.StdEncoding.DecodeString(req.RawBody)
		if err != nil {
			return nil, fmt.Errorf("raw_body: %w", err)
		}
		r.Body, r.ContentLength = raw, int64(len(raw))
	case len(req.Body) > 0 && string(req.Body) != "null":
		r.Body = []byte(req.Body)
	}
	resp, err := harness.DoWithStepUp(ctx, c, r)
	if err != nil {
		return nil, err
	}
	result := callResult{Status: resp.Status}
	if json.Valid(resp.Body) {
		result.Body = resp.Body
	}
	if secret := createdSecret(resp.Body); secret != "" {
		p.remember(secret)
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	p.calls++
	file, err := p.writeSecret(fmt.Sprintf("call-%d-%s.json", p.calls, req.OperationID), string(raw))
	if err != nil {
		return nil, err
	}
	return answer{"op": "call", "status": resp.Status, "file": file}, nil
}

// fillPath replaces each {name} of template by the escaped value of params.
func fillPath(template string, params map[string]string) (string, error) {
	var missing []string
	out := pathParam.ReplaceAllStringFunc(template, func(m string) string {
		name := m[1 : len(m)-1]
		v, ok := params[name]
		if !ok || v == "" {
			missing = append(missing, name)
			return m
		}
		return url.PathEscape(v)
	})
	if len(missing) > 0 {
		return "", fmt.Errorf("path_params misses %s", strings.Join(missing, ", "))
	}
	return out, nil
}

// createdSecret returns the member secret of an answer (createToken), so later answers are
// redacted against it.
func createdSecret(body []byte) string {
	var v struct {
		Secret string `json:"secret"`
	}
	if json.Unmarshal(body, &v) != nil {
		return ""
	}
	return v.Secret
}
