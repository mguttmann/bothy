// SPDX-License-Identifier: Apache-2.0

//go:build unix

package httpapi_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/testcontainers/testcontainers-go"
	tclog "github.com/testcontainers/testcontainers-go/log"
	"github.com/testcontainers/testcontainers-go/wait"
)

// The SeaweedFS of the object store tests of this package (M7-22): the image the S3 driver of
// the files module pins (internal/files/adapters/s3/seaweed_test.go), read from that file, so
// one pin moves both. Buckets are created with weed shell inside the container, so this package
// needs no S3 client of its own (only internal/audit/adapters/s3sink imports the AWS SDK).
const (
	seaweedS3Port       = "8333/tcp"
	seaweedStartTimeout = 3 * time.Minute
)

// seaweedServer is the started container with its S3 endpoint and keys.
type seaweedServer struct {
	ctr            *testcontainers.DockerContainer
	endpoint       string
	access, secret string
}

var seaweed struct {
	mu  sync.Mutex
	srv *seaweedServer
	err error
}

// sharedSeaweed returns the SeaweedFS of this test binary, starting it on first use.
func sharedSeaweed(t *testing.T) *seaweedServer {
	t.Helper()
	seaweed.mu.Lock()
	defer seaweed.mu.Unlock()
	if seaweed.srv == nil && seaweed.err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), seaweedStartTimeout)
		seaweed.srv, seaweed.err = startSeaweed(ctx)
		cancel()
	}
	if seaweed.err != nil {
		t.Fatalf("SeaweedFS: %v", seaweed.err)
	}
	return seaweed.srv
}

// hexToken returns n random bytes as hex.
func hexToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// startSeaweed starts the pinned image on 127.0.0.1 with a random host port and generated keys.
func startSeaweed(ctx context.Context) (*seaweedServer, error) {
	_, self, _, _ := runtime.Caller(0)
	pinFile := filepath.Join(filepath.Dir(self), "..", "..", "..", "files", "adapters", "s3", "seaweed_test.go")
	src, err := os.ReadFile(pinFile) //nolint:gosec // G304: a fixed file of the repository
	if err != nil {
		return nil, err
	}
	m := regexp.MustCompile(`(?m)^const seaweedImage = "([^"]+@sha256:[0-9a-f]{64})"$`).FindSubmatch(src)
	if m == nil {
		return nil, fmt.Errorf("no pinned seaweedImage in %s", pinFile)
	}
	name, err := hexToken(6)
	if err != nil {
		return nil, err
	}
	keyID, err := hexToken(6)
	if err != nil {
		return nil, err
	}
	key, err := hexToken(16)
	if err != nil {
		return nil, err
	}
	access, secret := "AKEXAMPLE"+strings.ToUpper(keyID), "EXAMPLE"+key
	port := network.MustParsePort(seaweedS3Port)
	ctr, err := testcontainers.Run(ctx, string(m[1]),
		testcontainers.WithLogger(tclog.NewNoopLogger()),
		testcontainers.WithName("bothy-seaweedfs-"+name),
		testcontainers.WithLabels(map[string]string{"org.bothy.test": "seaweedfs"}),
		testcontainers.WithEnv(map[string]string{"AWS_ACCESS_KEY_ID": access, "AWS_SECRET_ACCESS_KEY": secret}),
		testcontainers.WithEntrypoint("weed"),
		testcontainers.WithCmd("server", "-dir=/data", "-s3", "-ip.bind=0.0.0.0",
			"-master.volumeSizeLimitMB=32", "-volume.max=1000"),
		testcontainers.WithExposedPorts(seaweedS3Port),
		testcontainers.WithHostConfigModifier(func(hc *container.HostConfig) {
			hc.PortBindings = network.PortMap{port: {{HostIP: netip.MustParseAddr("127.0.0.1"), HostPort: "0"}}}
			hc.AutoRemove = true
		}),
		testcontainers.WithWaitStrategy(wait.ForHTTP("/").WithPort(seaweedS3Port).
			WithStatusCodeMatcher(func(status int) bool { return status == 403 }).
			WithStartupTimeout(seaweedStartTimeout)),
	)
	if err != nil {
		if ctr != nil {
			_ = testcontainers.TerminateContainer(ctr)
		}
		return nil, fmt.Errorf("start SeaweedFS: %w", err)
	}
	mapped, err := ctr.MappedPort(ctx, seaweedS3Port)
	if err != nil {
		_ = testcontainers.TerminateContainer(ctr)
		return nil, fmt.Errorf("mapped port: %w", err)
	}
	return &seaweedServer{ctr: ctr, endpoint: fmt.Sprintf("http://127.0.0.1:%d", mapped.Num()), access: access, secret: secret}, nil
}

// stopSeaweed removes the container of the test binary, if one was started.
func stopSeaweed() error {
	seaweed.mu.Lock()
	defer seaweed.mu.Unlock()
	srv := seaweed.srv
	seaweed.srv, seaweed.err = nil, nil
	if srv == nil {
		return nil
	}
	if err := testcontainers.TerminateContainer(srv.ctr); err != nil {
		return fmt.Errorf("terminate container %s: %w", srv.ctr.GetContainerID(), err)
	}
	return nil
}

// newLockedBucket creates a bucket with Object Lock (and so versioning) through weed shell and
// returns its name. The filer may still be starting, so creation is tried a few times.
func (s *seaweedServer) newLockedBucket(t *testing.T) string {
	t.Helper()
	suffix, err := hexToken(6)
	if err != nil {
		t.Fatal(err)
	}
	name := "bothy-audit-" + suffix
	var out string
	for range 30 {
		code, r, err := s.ctr.Exec(t.Context(), []string{"sh", "-c", "echo 's3.bucket.create -name " + name + " -withLock' | weed shell"})
		if err == nil {
			b, _ := io.ReadAll(r)
			out = string(b)
			if code == 0 && !strings.Contains(strings.ToLower(out), "error") {
				return name
			}
		}
		time.Sleep(time.Second)
	}
	t.Fatalf("weed shell s3.bucket.create: %s", out)
	return ""
}
