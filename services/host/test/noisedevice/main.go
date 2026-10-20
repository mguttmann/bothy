// SPDX-License-Identifier: Apache-2.0

// Command noisedevice is the Noise sidecar of the Go test device of the terminal suite (brief
// M2-48, Design note 6). The server module cannot import internal/noise of the host service, so
// the device in services/server/test/terminal/device keeps HTTP, DPoP and WebSocket and runs the
// handshake and the frames through this process, which uses internal/noise and internal/frame
// unchanged.
//
// Protocol: one JSON object per line on stdin, one answer line per request on stdout, one
// process per device session; bytes are base64url without padding. keypair (optional, before
// start) draws the ephemeral key first, because openTerminalSession needs its hash before the
// device knows the session ID of the prologue; start keeps it.
//
//	{"op":"keypair"}                                          -> {"ok":true,"ephemeral_public":B,"cek":S}
//	{"op":"start","host_static":B,"session_id":U,"host_id":U} -> {"ok":true,"ephemeral_public":B,"cek":S}
//	{"op":"message1","payload":B}                             -> {"ok":true,"message":B}
//	{"op":"message2","message":B}                             -> {"ok":true,"payload":B}
//	{"op":"seal","frame":B}                                   -> {"ok":true,"message":B}
//	{"op":"open","message":B}                                 -> {"ok":true,"frame":B}
//
// A failure answers {"ok":false,"error":{"kind":K}}, K being the Go name of the sentinel error of
// internal/noise or internal/frame (for example ErrHandshake, ErrWrongDirection) or one of
// bad_json, unknown_op, out_of_order, bad_field and internal; the process then serves the next
// line. EOF on stdin ends it with exit code 0. stderr stays empty except for one fixed line
// without data when reading stdin or writing stdout fails (exit code 1).
package main

import "os"

func main() {
	os.Exit(serve(os.Stdin, os.Stdout, os.Stderr))
}
