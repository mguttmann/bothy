// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"bufio"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
)

// asyncAPIPath is the gateway contract, relative to this package.
const asyncAPIPath = "../../../../../docs/api/asyncapi.yaml"

// gatewayCloseCodes reads the x-close-codes of the channel gateway from the AsyncAPI document: a
// line scan, because the domain layer imports no YAML library. The list ends at the next channel.
func gatewayCloseCodes(t *testing.T) map[int]string {
	t.Helper()
	f, err := os.Open(asyncAPIPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	out := map[int]string{}
	inGateway, inCodes, code := false, false, 0
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case line == "  gateway:":
			inGateway = true
		case inGateway && strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "   ") && line != "  gateway:":
			return out
		case inGateway && strings.TrimSpace(line) == "x-close-codes:":
			inCodes = true
		case inCodes && strings.HasPrefix(strings.TrimSpace(line), "- code:"):
			code, err = strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "- code:")))
			if err != nil {
				t.Fatalf("code line %q", line)
			}
		case inCodes && strings.HasPrefix(strings.TrimSpace(line), "name:") && code != 0:
			out[code] = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "name:"))
			code = 0
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// TestCloseCodesMatchAsyncAPI: every code of x-close-codes of the channel gateway has a constant
// with the same name and every constant is in the list (04 section 6.4).
func TestCloseCodesMatchAsyncAPI(t *testing.T) {
	t.Parallel()
	want := gatewayCloseCodes(t)
	if len(want) == 0 {
		t.Fatal("no close codes read")
	}
	got := map[int]string{}
	for _, c := range domain.CloseCodes() {
		got[int(c)] = c.Name()
	}
	for code, name := range want {
		if got[code] != name {
			t.Errorf("code %d: constant name %q, contract %q", code, got[code], name)
		}
	}
	for code, name := range got {
		if _, ok := want[code]; !ok {
			t.Errorf("constant %d (%s) is not in the contract", code, name)
		}
	}
	if !slices.IsSorted(domain.CloseCodes()) {
		t.Error("CloseCodes is not sorted")
	}
	if domain.CloseCode(4010).Name() != "" {
		t.Error("an unknown code has a name")
	}
}

// TestCloseCodeOutcome: the outcome of a close for bothy.outcome (signals.md "gateway"): the
// client codes are client_error, 4000 and 4013 server_error, everything else ok after ready and
// server_error before it; a client that leaves before ready is client_error.
func TestCloseCodeOutcome(t *testing.T) {
	t.Parallel()
	for _, c := range []domain.CloseCode{4001, 4002, 4003, 4004, 4005, 4006, 4007, 4008, 4011, 4012, 4016, 4017, 4018} {
		if got := domain.Outcome(c, false); got != domain.OutcomeClientError {
			t.Errorf("%d before ready: %s", c, got)
		}
	}
	if got := domain.Outcome(domain.CloseUnknownError, true); got != domain.OutcomeServerError {
		t.Errorf("4000: %s", got)
	}
	if got := domain.Outcome(domain.CloseHeartbeatTimeout, true); got != domain.OutcomeOK {
		t.Errorf("4009 after ready: %s", got)
	}
	if got := domain.Outcome(0, false); got != domain.OutcomeClientError {
		t.Errorf("client gone before ready: %s", got)
	}
	if got := domain.Outcome(domain.CloseSlowConsumer, false); got != domain.OutcomeServerError {
		t.Errorf("4014 before ready: %s", got)
	}
}
