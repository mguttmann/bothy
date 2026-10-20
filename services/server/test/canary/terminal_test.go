// SPDX-License-Identifier: Apache-2.0

package canary

import (
	"strings"
	"testing"
)

// TestSEC_T10_TerminalInputOutput proves SEC-T10 for terminal content (brief M2-46 Design note 13
// and requirement 24, threat T-TM-01): the terminal step of the main scenario runs a shell whose
// device and Fake-Host speak Noise_NK_25519_ChaChaPoly_SHA256; a canary input and a canary output
// pass the relay encrypted. The positive controls hold (the input arrives in clear at the
// Fake-Host, the output at the device, the session token is the payload of message 1), the raw
// relay messages are a sink of the run, and no terminal canary appears in any sink:
// TestSEC_T10_CanaryAcrossAllOutputs scans the same sinks with every canary of the run.
func TestSEC_T10_TerminalInputOutput(t *testing.T) {
	res := scenario(t)
	io := res.sc.TerminalIO
	if !io.InputAtHost || !io.OutputAtDevice || !io.TokenInMessage1 || !io.Closed {
		t.Fatalf("terminal step: input at the host %v, output at the device %v, token in message 1 %v, closed %v; want all",
			io.InputAtHost, io.OutputAtDevice, io.TokenInMessage1, io.Closed)
	}
	if n := res.run.Sinks.Count(SinkRelay); n < io.RelayMessages || io.RelayMessages < 5 {
		t.Fatalf("relay sink: %d records of %d messages; want the handshake and the frames", n, io.RelayMessages)
	}
	t.Logf("terminal step: %+v, relay records %d", io, res.run.Sinks.Count(SinkRelay))
	var f []Finding
	for _, x := range Scan(res.run.Reg, res.run.Sinks, AllowRules{Readable: res.sc.Readable}) {
		if strings.HasPrefix(x.Label, "terminal ") {
			f = append(f, x)
		}
	}
	if len(f) > 0 {
		t.Errorf("%d findings of terminal canaries:\n%s", len(f), Report(f))
	}
}
