// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"time"
)

// Defaults of healthcheck: the TLS server of the container backup as the entry point of the image
// configures it, checked with the stack's own CA and the certificate of backup
// (docs/operations/configuration.md section 5, pgbackrest_tls/).
const (
	defaultHealthAddr       = "127.0.0.1:8432"
	defaultHealthServerName = "backup"
	defaultTLSDir           = "/run/bothy/secrets/pgbackrest_tls/"
	// defaultHealthTimeout matches the timeout of the health check of backup in
	// infra/compose/compose.yaml.
	defaultHealthTimeout = 5 * time.Second
)

// healthcheckCommand runs "bothy-backupd healthcheck" with the arguments after the command name:
// a TLS handshake with the pgBackRest server at --addr, with the client certificate --cert and
// --key, accepting only a server certificate for --server-name that chains to --ca. It returns
// 0 when the handshake completes, 1 with one line on stderr when it does not, and 2 for a usage
// error. It sends no pgBackRest command, so a check never touches the repository.
func healthcheckCommand(ctx context.Context, args []string, stderr io.Writer) int {
	fs := flag.NewFlagSet("bothy-backupd healthcheck", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	addr := fs.String("addr", defaultHealthAddr, "TLS server")
	name := fs.String("server-name", defaultHealthServerName, "expected server name")
	var files tlsFiles
	fs.StringVar(&files.ca, "ca", defaultTLSDir+"ca.crt", "CA certificate")
	fs.StringVar(&files.cert, "cert", defaultTLSDir+"backup.crt", "client certificate")
	fs.StringVar(&files.pair, "key", defaultTLSDir+"backup.key", "client key") // gitleaks:allow (a file path, no key)
	timeout := fs.Duration("timeout", defaultHealthTimeout, "limit of the check")
	if err := fs.Parse(args); err != nil {
		return usageError(stderr, "healthcheck: "+err.Error())
	}
	switch {
	case fs.NArg() > 0:
		return usageError(stderr, fmt.Sprintf("healthcheck: unexpected argument %q", fs.Arg(0)))
	case *timeout <= 0:
		return usageError(stderr, "healthcheck: --timeout must be positive")
	case !loopbackAddr(*addr):
		return usageError(stderr, "healthcheck: --addr must be a loopback address with a port")
	}
	cfg, err := clientTLS(files, *name)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "bothy-backupd healthcheck: %v\n", err)
		return exitFailure
	}
	ctx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()
	// Not HTTP and only to loopback (loopbackAddr): the probe of the container's own TLS server,
	// which the egress package (HTTP only) does not cover.
	d := tls.Dialer{NetDialer: &net.Dialer{}, Config: cfg} // nosemgrep: bothy-go-egress-only -- a TLS handshake with the pgBackRest server on loopback, no HTTP and no egress
	conn, err := d.DialContext(ctx, "tcp", *addr)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "bothy-backupd healthcheck: %s\n", dialErrorClass(err))
		return exitFailure
	}
	_ = conn.Close()
	return exitOK
}

// tlsFiles are the paths of the PEM files of healthcheck.
type tlsFiles struct {
	ca   string // CA certificate, the only root
	cert string // client certificate
	pair string // private key of the client certificate
}

// loopbackAddr reports whether addr is host:port with a loopback IP address (127.0.0.0/8, ::1)
// or localhost: the health check probes only the container's own server.
func loopbackAddr(addr string) bool {
	host, port, err := net.SplitHostPort(addr)
	if err != nil || port == "" {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip, err := netip.ParseAddr(host)
	return err == nil && ip.Unmap().IsLoopback()
}

// clientTLS builds the client configuration: the CA as the only root and the client key pair.
func clientTLS(files tlsFiles, name string) (*tls.Config, error) {
	pair, err := tls.LoadX509KeyPair(files.cert, files.pair)
	if err != nil {
		return nil, fmt.Errorf("client certificate: %w", err)
	}
	pemCA, err := os.ReadFile(files.ca)
	if err != nil {
		return nil, fmt.Errorf("CA certificate: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pemCA) {
		return nil, errors.New("CA certificate: no PEM certificate in the file")
	}
	return &tls.Config{
		RootCAs: pool, Certificates: []tls.Certificate{pair}, ServerName: name, MinVersion: tls.VersionTLS12,
	}, nil
}

// dialErrorClass names why the handshake failed without repeating addresses or certificate
// contents: a refused or timed-out connection, or a certificate the CA does not vouch for.
func dialErrorClass(err error) string {
	var unknown x509.UnknownAuthorityError
	var hostname x509.HostnameError
	var invalid x509.CertificateInvalidError
	var verify *tls.CertificateVerificationError
	switch {
	case errors.As(err, &unknown), errors.As(err, &hostname), errors.As(err, &invalid), errors.As(err, &verify):
		return "the server certificate is not one of this stack: " + err.Error()
	case errors.Is(err, context.DeadlineExceeded):
		return "no answer within the timeout"
	}
	var op *net.OpError
	if errors.As(err, &op) && op.Op == "dial" {
		return "no answer: " + err.Error()
	}
	return "handshake failed: " + err.Error()
}
