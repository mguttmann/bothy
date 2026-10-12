// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestSwitchLoggerFollowsTheLastLogger(t *testing.T) {
	t.Parallel()
	var first, second bytes.Buffer
	sw := newSwitchLogger(slog.New(slog.NewJSONHandler(&first, nil)))
	derived := slog.New(sw).With(slog.String("k", "v")).WithGroup("g")
	derived.Info("one", slog.Int("n", 1))
	sw.set(slog.New(slog.NewJSONHandler(&second, &slog.HandlerOptions{Level: slog.LevelWarn})))
	derived.Info("dropped by the level of the second logger")
	derived.Warn("two", slog.Int("n", 2))
	if !strings.Contains(first.String(), `"msg":"one","k":"v","g":{"n":1}`) {
		t.Fatalf("first = %s", first.String())
	}
	if !strings.Contains(second.String(), `"msg":"two","k":"v","g":{"n":2}`) || strings.Contains(second.String(), "dropped") {
		t.Fatalf("second = %s", second.String())
	}
	if sw.Enabled(context.Background(), slog.LevelInfo) {
		t.Fatal("Enabled ignores the level of the current logger")
	}
}
