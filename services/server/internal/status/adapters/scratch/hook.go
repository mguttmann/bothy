// SPDX-License-Identifier: Apache-2.0

//go:build !restoretesthook

package scratch

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// afterStart is the test hook after the copy started; production builds have none (brief M7-03
// design note 3). The build tag restoretesthook replaces it (hook_restoretesthook.go).
func afterStart(context.Context, string, *pg.Pool) error { return nil }

// afterFetch is the test hook after the objects were restored; production builds have none.
func afterFetch(context.Context, string) error { return nil }
