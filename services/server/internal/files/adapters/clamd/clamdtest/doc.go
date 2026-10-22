// SPDX-License-Identifier: Apache-2.0

// Package clamdtest holds the test doubles of the clamd adapter; only tests import it
// (docs/DECISIONS.md D-M3-04-13).
//
//   - Fake is a scripted clamd on a Unix socket in a temporary directory: replies, delays,
//     stalls and early closes, with a record of every command and chunk.
//   - Start runs the pinned clamd image (Image, the version of BOTHY_CLAMAV_VERSION in
//     infra/compose/.env.example) as a throwaway container: clamd in the foreground with a test
//     configuration (AlertExceedsMax yes, small explicit limits) and a database of one hash
//     signature for the EICAR test string computed at run time, so no signature is downloaded and
//     the full database never loads (RK-52). Docker runtimes on a VM do not pass Unix sockets of
//     bind mounts to the host, so the container listens on a loopback TCP port as well and a
//     relay offers it on a Unix socket of the host: the production client, which accepts only
//     unix:// addresses, talks to the real clamd unchanged.
//
// Example:
//
//	srv, err := clamdtest.Start(ctx, clamdtest.Options{})
//	if err != nil {
//		t.Fatal(err)
//	}
//	defer srv.Terminate(context.Background())
//	c, _ := clamd.New(clamd.Config{Address: srv.Address(), ChunkBytes: 4096, Timeout: time.Minute, MaxConcurrent: 2})
package clamdtest
