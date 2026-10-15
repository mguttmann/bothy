// SPDX-License-Identifier: Apache-2.0

package webhookhttp_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/OWNER/bothy/services/server/internal/apps/adapters/webhookhttp"
	"github.com/OWNER/bothy/services/server/internal/apps/app"
	"github.com/OWNER/bothy/services/server/internal/apps/domain"
	"github.com/OWNER/bothy/services/server/internal/platform/buildinfo"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
)

const host = "hooks.example.com"

// resolver answers every name with one address.
type resolver struct{ addr string }

func (r resolver) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) {
	return []netip.Addr{netip.MustParseAddr(r.addr)}, nil
}

// receiver starts a TLS server for hooks.example.com that answers with handler.
func receiver(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: host}, DNSNames: []string{host},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, _ := x509.ParseCertificate(der)
	pool := x509.NewCertPool()
	pool.AddCert(cert)
	s := httptest.NewUnstartedServer(handler)
	s.TLS = &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}, MinVersion: tls.VersionTLS12}
	s.StartTLS()
	t.Cleanup(s.Close)
	return s, pool
}

// sender builds the sender for the server s with allowPrivate on its rule.
func sender(t *testing.T, s *httptest.Server, pool *x509.CertPool, allowPrivate bool) (*webhookhttp.Sender, string, *tracetest.SpanRecorder) {
	t.Helper()
	_, port, _ := net.SplitHostPort(s.Listener.Addr().String())
	hosts, err := domain.ParseAllowedHosts([]string{host + ":" + port})
	if err != nil {
		t.Fatal(err)
	}
	cfg := egress.Config{Timeout: 5 * time.Second, MaxResponseBytes: 1 << 20}
	if allowPrivate {
		rule, err := egress.ParseRule(host + ":" + port + ";allow_private")
		if err != nil {
			t.Fatal(err)
		}
		cfg.AllowedHosts = []egress.Rule{rule}
	}
	rec := tracetest.NewSpanRecorder()
	snd, err := webhookhttp.New(cfg, hosts, 5*time.Second, egress.Deps{Resolver: resolver{"127.0.0.1"}, RootCAs: pool},
		sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec)))
	if err != nil {
		t.Fatal(err)
	}
	return snd, "https://" + host + ":" + port + "/chat?k=v", rec
}

// request is one delivery to url.
func request(url string) app.SignedRequest {
	return app.SignedRequest{URL: url, Host: host, WebhookID: "0192f4a0-7b1c-757c-87c8-267db4b2c89f", Timestamp: 1760346000,
		Signature: "v1a,AQID", Body: []byte(`{"id":"x"}`), EventSeq: 7, EventType: "message.created"}
}

// spanAttrs returns the attributes of the one ended span.
func spanAttrs(t *testing.T, rec *tracetest.SpanRecorder) map[attribute.Key]attribute.Value {
	t.Helper()
	spans := rec.Ended()
	if len(spans) != 1 || spans[0].Name() != webhookhttp.SpanName {
		t.Fatalf("spans %d", len(spans))
	}
	out := map[attribute.Key]attribute.Value{}
	for _, kv := range spans[0].Attributes() {
		out[kv.Key] = kv.Value
	}
	return out
}

// TestSenderHeadersAndOutcome: one POST with the headers of Standard Webhooks, the user agent of the
// server and the body as signed; the status and Retry-After come back, the body is never read; the
// span carries host, status and outcome and nothing of the path.
func TestSenderHeadersAndOutcome(t *testing.T) {
	t.Parallel()
	var got *http.Request
	var body []byte
	s, pool := receiver(t, func(w http.ResponseWriter, r *http.Request) {
		got, body = r.Clone(context.Background()), mustRead(r)
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, strings.Repeat("ignored body ", 100))
	})
	snd, url, rec := sender(t, s, pool, true)
	res, err := snd.Send(context.Background(), request(url))
	if err != nil || res.StatusCode != http.StatusServiceUnavailable || res.RetryAfter != "30" {
		t.Fatalf("Send = %+v %v", res, err)
	}
	if got.Method != http.MethodPost || got.Header.Get("Content-Type") != "application/json" || got.Header.Get("webhook-id") != request(url).WebhookID ||
		got.Header.Get("webhook-timestamp") != "1760346000" || got.Header.Get("webhook-signature") != "v1a,AQID" ||
		got.Header.Get("User-Agent") != "chat-server-webhooks/"+buildinfo.Version() || string(body) != `{"id":"x"}` {
		t.Fatalf("request %v %s", got.Header, body)
	}
	a := spanAttrs(t, rec)
	if a[attr.ServerAddress].AsString() != host || a[attr.HTTPResponseStatusCode].AsInt64() != 503 || a[attr.Outcome].AsString() != webhookhttp.OutcomeThrottled {
		t.Fatalf("span %v", a)
	}
	for _, v := range a {
		if strings.Contains(v.Emit(), "/chat") || strings.Contains(v.Emit(), "k=v") {
			t.Fatalf("span carries the path: %v", a)
		}
	}
}

// TestSenderOutcomes maps answers to bothy.outcome.
func TestSenderOutcomes(t *testing.T) {
	t.Parallel()
	for status, want := range map[int]string{200: webhookhttp.OutcomeSucceeded, 410: webhookhttp.OutcomeGone, 500: webhookhttp.OutcomeFailed,
		302: webhookhttp.OutcomeFailed} {
		s, pool := receiver(t, func(w http.ResponseWriter, _ *http.Request) {
			if status == 302 {
				w.Header().Set("Location", "https://alt.example.com/other")
			}
			w.WriteHeader(status)
		})
		snd, url, rec := sender(t, s, pool, true)
		res, err := snd.Send(context.Background(), request(url))
		if err != nil || res.StatusCode != status {
			t.Fatalf("%d: %+v %v", status, res, err)
		}
		if got := spanAttrs(t, rec)[attr.Outcome].AsString(); got != want {
			t.Fatalf("%d: outcome %s, want %s", status, got, want)
		}
	}
}

// TestSenderBlockedTarget: without allow_private the loopback target is refused before any
// connection; the error is a *domain.BlockedError with address_blocked, names no path, and the
// span outcome is blocked.
func TestSenderBlockedTarget(t *testing.T) {
	t.Parallel()
	called := false
	s, pool := receiver(t, func(w http.ResponseWriter, _ *http.Request) { called = true })
	snd, url, rec := sender(t, s, pool, false)
	_, err := snd.Send(context.Background(), request(url))
	var blocked *domain.BlockedError
	if !errors.As(err, &blocked) || blocked.Reason != string(egress.ReasonAddressBlocked) || called {
		t.Fatalf("Send = %v, called %v", err, called)
	}
	if strings.Contains(err.Error(), "/chat") || strings.Contains(err.Error(), "k=v") {
		t.Fatalf("error names the path: %v", err)
	}
	if got := spanAttrs(t, rec)[attr.Outcome].AsString(); got != webhookhttp.OutcomeBlocked {
		t.Fatalf("outcome %s", got)
	}
	if o := domain.Classify(0, err); o.Error != domain.LastErrorAddressBlocked {
		t.Fatalf("classified %+v", o)
	}
}

// TestSenderTransportFailure: a failed TLS check is a transport error the domain classifies as
// tls_failed, with outcome failed.
func TestSenderTransportFailure(t *testing.T) {
	t.Parallel()
	s, _ := receiver(t, func(http.ResponseWriter, *http.Request) {})
	snd, url, rec := sender(t, s, x509.NewCertPool(), true)
	_, err := snd.Send(context.Background(), request(url))
	if err == nil || domain.Classify(0, err).Error != domain.LastErrorTLSFailed {
		t.Fatalf("Send = %v, classified %+v", err, domain.Classify(0, err))
	}
	if got := spanAttrs(t, rec)[attr.Outcome].AsString(); got != webhookhttp.OutcomeFailed {
		t.Fatalf("outcome %s", got)
	}
}

// mustRead reads the body of r.
func mustRead(r *http.Request) []byte {
	b, _ := io.ReadAll(r.Body)
	return b
}
