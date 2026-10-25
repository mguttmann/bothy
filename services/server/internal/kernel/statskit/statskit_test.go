// SPDX-License-Identifier: Apache-2.0

package statskit_test

import (
	"errors"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/statskit"
)

func TestCheckQuery(t *testing.T) {
	t.Parallel()
	for _, g := range []string{"hour", "day", "week", "month"} {
		if err := statskit.CheckQuery(g, "Europe/Berlin", 2, 2); err != nil {
			t.Errorf("CheckQuery(%q) = %v, want nil", g, err)
		}
	}
	cases := map[string]struct {
		granularity, tz string
		keys, groups    int
	}{
		"unknown granularity": {"year", "UTC", 0, 0},
		"empty time zone":     {"day", "", 0, 0},
		"local time zone":     {"day", "Local", 0, 0},
		"unknown time zone":   {"day", "Mars/Olympus", 0, 0},
		"lists differ":        {"day", "UTC", 2, 1},
	}
	for name, c := range cases {
		err := statskit.CheckQuery(c.granularity, c.tz, c.keys, c.groups)
		var e *errs.Error
		if !errors.As(err, &e) || e.Code != errs.CodeValidationFailed {
			t.Errorf("%s: CheckQuery = %v, want validation_failed", name, err)
		}
	}
}

func TestReadError(t *testing.T) {
	t.Parallel()
	own := errs.New(errs.CodeValidationFailed, ": bad")
	if got, ok := errs.As(statskit.ReadError(own, "counting")); !ok || got != own {
		t.Errorf("ReadError(errs error) = %v, want it unchanged", got)
	}
	raw := errors.New("connection reset")
	got := statskit.ReadError(raw, "counting runs")
	e, ok := errs.As(got)
	if !ok || e.Code != errs.CodeInternalError || !errors.Is(got, raw) {
		t.Errorf("ReadError(raw) = %v, want internal_error wrapping it", got)
	}
}
