// SPDX-License-Identifier: Apache-2.0

// Package clamd is the Scanner adapter of the files module: a small client of the clamd protocol
// (docs/spec/12-files.md 5.1, ADR 0033) over a Unix socket only (19 SEC-TLS-5). It speaks
// zPING and zINSTREAM with chunks of at most file.scan.chunk_bytes, each preceded by its length
// in four bytes of network byte order and ended by a chunk of length zero, and reads the reply
// until clamd closes the connection. At most file.scan.max_concurrent scans run at once per
// process. It uses the standard library only: no go-clamd, no cgo scanner library.
//
// The adapter logs nothing and sets no span attribute, so neither a signature, a reply text nor
// a file name can leave it outside the verdict (12 14).
//
// Example:
//
//	c, err := clamd.New(clamd.Config{Address: "unix:///run/clamav/clamd.sock", ChunkBytes: 65536,
//		Timeout: 2 * time.Minute, MaxConcurrent: 4})
//	if err != nil {
//		return err
//	}
//	v, err := c.Scan(ctx, body)
package clamd
