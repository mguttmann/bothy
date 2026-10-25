// SPDX-License-Identifier: Apache-2.0

// Command termsuite-images builds the server and host images of the terminal suite
// (test/terminal/stack) with exactly the builds stack.Start runs, so the TestMain of every e2e
// package of the suite that starts afterwards on the same Docker daemon finds them in the
// BuildKit cache instead of building them again. tools/ci/go_shards.py runs it before the tests of
// a shard that holds such a package (PL/39): built inside the tests, the server image took five
// to ten minutes of every e2e package and competed with the other packages of the shard for the
// CPUs. It prints the tags and the build times; the exit code is 1 on any failure.
//
// Usage (from services/server): go run ./test/terminal/cmd/termsuite-images
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/OWNER/bothy/services/server/test/terminal/stack"
)

func main() {
	if err := run(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "termsuite-images: %v\n", err)
		os.Exit(1)
	}
}

// run checks the Docker runtime and builds both images.
func run(ctx context.Context) error {
	root, err := stack.RepoRoot()
	if err != nil {
		return err
	}
	d := &stack.Docker{}
	if err := stack.Preflight(ctx, d); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp("", "termsuite-images-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	t0 := time.Now()
	server, err := stack.ServerImage(ctx, d, root, tmp)
	if err != nil {
		return fmt.Errorf("server image: %w", err)
	}
	fmt.Printf("termsuite-images: server image %s built in %s\n", server, time.Since(t0).Round(time.Second))
	t0 = time.Now()
	host, err := stack.HostImage(ctx, d, root)
	if err != nil {
		return fmt.Errorf("host image: %w", err)
	}
	fmt.Printf("termsuite-images: host image %s built in %s\n", host, time.Since(t0).Round(time.Second))
	return nil
}
