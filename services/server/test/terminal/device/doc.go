// SPDX-License-Identifier: Apache-2.0

// Package device is the Go test device of the terminal conformance suite (brief M2-48 Design
// note 6): it keeps HTTP, DPoP and the WebSocket of the device stream in the server module and runs
// the Noise handshake and the frames in the sidecar services/host/test/noisedevice, which uses the
// host service's own internal/noise and internal/frame. Identity comes from the harness client of
// M1-71 (test/harness); the Dialer maps the instance origin https://chat.example.com:8443 to the
// published port of the TLS front and trusts only the test CA of the stack. The device keeps every
// binary message it sent and received in memory (the relay view of TM-T06) and can cut its TCP
// connection without a close frame (TM-T16). It belongs to the architecture component
// testsupport and is never imported by production code.
package device
