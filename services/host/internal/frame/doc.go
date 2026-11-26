// SPDX-License-Identifier: Apache-2.0

// Package frame encodes and decodes the frames of the terminal stream
// (docs/spec/24-terminal.md 5.4): after the Noise handshake every transport message carries one
// frame, one type byte and a body. The same frames run over the local channel between the host
// service and the PTY helper (M2-42). The package does no I/O, writes no logs and depends only
// on the standard library; its errors name the type byte and lengths, never the body.
//
// Byte order, body rules and directions are the rulings of docs/DECISIONS.md D-M2-38-10: all
// integers big-endian; start and resize carry columns then rows, each 1 to 65535; data carries
// at least one byte; close, replay_begin and replay_end carry no body; exit carries a signed
// 32-bit exit code. Every decode error ends the session like an unknown type (close code 4104);
// M2-45 maps the errors.
//
// A computer session (docs/spec/27-computer-use.md 6.3) uses the same framing with close and the
// types 0x20 to 0x25: one JSON object without unknown members for cu_action, cu_result,
// cu_image_begin and cu_state, raw image bytes for cu_image_data, no body for cu_image_end. Decode
// takes the session kind, so a computer frame on a terminal session and a terminal frame on a
// computer session are both an unknown type.
package frame
