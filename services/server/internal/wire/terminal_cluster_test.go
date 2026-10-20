// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/config"
)

// TestClusterModeKey (M2-47 Design note 2): cluster.mode is the enum single or multi with the
// default of 02 section 22; another value fails the start with a message naming the key.
func TestClusterModeKey(t *testing.T) {
	t.Parallel()
	v, err := config.Load(schema(t), config.Sources{Env: testEnv()})
	if err != nil {
		t.Fatal(err)
	}
	if got := v.String("cluster.mode"); got != "single" {
		t.Fatalf("default %q", got)
	}
	v, err = config.Load(schema(t), config.Sources{Env: append(testEnv(), "BOTHY_CLUSTER__MODE=multi")})
	if err != nil || v.String("cluster.mode") != "multi" {
		t.Fatalf("multi: %v", err)
	}
	_, err = config.Load(schema(t), config.Sources{Env: append(testEnv(), "BOTHY_CLUSTER__MODE=several")})
	if err == nil || !strings.Contains(err.Error(), "cluster.mode") {
		t.Fatalf("an unknown mode: %v", err)
	}
}
