// SPDX-License-Identifier: Apache-2.0

//go:build contract_validate

package httpserver_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// installChildEnv switches the test binary into the child of TestG8_ChainInstallsTheCheck.
const installChildEnv = "BOTHY_G8_INSTALL_CHILD"

// TestG8_ChainInstallsTheCheck: in the test build, Chain.Public of a chain over the repository
// contract checks every response; a getMe answer without its required members ends the process
// with exit code 3 and the report of contractcheck (M1-71).
func TestG8_ChainInstallsTheCheck(t *testing.T) {
	if os.Getenv(installChildEnv) == "1" { //nolint:forbidigo // switch of the own child test process
		op := httpserver.Operation{ID: "getMe", Method: http.MethodGet, Path: "/api/v1/me", Pattern: "GET /api/v1/me", Action: "self.read"}
		h := httpservertest.New(t, httpservertest.Options{Contract: rest.Spec, Routes: []httpservertest.Route{{
			Op: op, Handler: httpservertest.Authorized("self.read", func(w http.ResponseWriter, _ *http.Request) {
				httpservertest.JSON(w, http.StatusOK, map[string]any{"is_owner": false})
			}),
		}}})
		h.Do(h.NewRequest(http.MethodGet, "/api/v1/me", "", "Authorization", httpservertest.GoodToken))
		t.Fatal("the response check did not end the process")
	}
	t.Parallel()
	cmd := exec.CommandContext(context.Background(), os.Args[0], "-test.run=^TestG8_ChainInstallsTheCheck$", "-test.count=1") //nolint:gosec // G204: the own test binary
	cmd.Env = append(os.Environ(), installChildEnv+"=1")                                                                      //nolint:forbidigo // the child test binary needs the environment of go test
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 3 {
		t.Fatalf("child: %v, want exit code 3; stderr:\n%s", err, stderr.String())
	}
	if !strings.Contains(stderr.String(), "operationId=getMe") || !strings.Contains(stderr.String(), "pointer=/principal") {
		t.Fatalf("report:\n%s", stderr.String())
	}
}

// inBandChildEnv switches the test binary into the child of TestG8_SkipsInBandRoutes.
const inBandChildEnv = "BOTHY_G8_INBAND_CHILD"

// TestG8_SkipsInBandRoutes: a route that authorizes in band (the control connection of the host
// services, GET /api/v1/hosts/connect) is no operation of the contract, so G8 must not check its
// answer against the contract operation whose pattern also matches the path (getHost, GET
// /api/v1/hosts/{host_id}); before INT-W45/3 its 426 ended the process.
func TestG8_SkipsInBandRoutes(t *testing.T) {
	if os.Getenv(inBandChildEnv) == "1" { //nolint:forbidigo // switch of the own child test process
		v, err := httpserver.NewValidator(rest.Spec, []string{"getHost"})
		if err != nil {
			t.Fatal(err)
		}
		chain, err := httpserver.NewChain(httpserver.DefaultChainConfig(httpservertest.PublicURL), httpserver.Deps{
			Logger: slog.New(slog.DiscardHandler), Authenticator: httpservertest.FakeAuthenticator{}, Validator: v,
		})
		if err != nil {
			t.Fatal(err)
		}
		op := httpserver.Operation{ID: "hostConnect", Method: http.MethodGet, Path: "/api/v1/hosts/connect",
			Pattern: "GET /api/v1/hosts/connect", Action: "host.connect", AuthorizedInBand: true}
		mux := http.NewServeMux()
		mux.Handle(op.Pattern, chain.Wrap(op, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "upgrade required", http.StatusUpgradeRequired)
		})))
		srv := httptest.NewServer(chain.Public(mux))
		defer srv.Close()
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/api/v1/hosts/connect", http.NoBody)
		resp, err := srv.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusUpgradeRequired {
			t.Fatalf("status %d, want 426", resp.StatusCode)
		}
		return
	}
	t.Parallel()
	cmd := exec.CommandContext(context.Background(), os.Args[0], "-test.run=^TestG8_SkipsInBandRoutes$", "-test.count=1") //nolint:gosec // G204: the own test binary
	cmd.Env = append(os.Environ(), inBandChildEnv+"=1")                                                                   //nolint:forbidigo // the child test binary needs the environment of go test
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("child: %v; stderr:\n%s", err, stderr.String())
	}
}
