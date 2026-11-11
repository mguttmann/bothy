// SPDX-License-Identifier: Apache-2.0

package install

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/host/internal/config"
)

// Ops are the operations an executor needs; the real ones run with administrator rights, the
// tests record them.
type Ops interface {
	// Run starts argv[0] (absolute) with the rest as arguments and an empty environment.
	Run(ctx context.Context, argv []string) error
	// Mkdir creates path (if missing) and sets mode and owner.
	Mkdir(path string, mode fs.FileMode, owner string) error
	// Write writes content to path when it is missing or differs, and sets mode and owner.
	Write(path string, content []byte, mode fs.FileMode, owner string) error
	// Copy copies src to dst and sets mode and owner.
	Copy(src, dst string, mode fs.FileMode, owner string) error
	// Chmod sets mode (and owner when not empty) of path.
	Chmod(path string, mode fs.FileMode, owner string) error
	// Remove removes path; a missing path is not an error.
	Remove(path string) error
	// Home returns the home of user (for "~user" paths).
	Home(user string) (string, error)
	// Windows applies a Windows step.
	Windows(ctx context.Context, s Step) error
}

// Apply executes steps in order and stops at the first failure; the error names the step, never
// a content.
func Apply(ctx context.Context, ops Ops, steps []Step) error {
	for _, s := range steps {
		if err := apply(ctx, ops, s); err != nil {
			what := s.Path + s.Name
			if what == "" && len(s.Argv) > 0 {
				// A run step names its program and its first argument, never a value.
				what = filepath.Base(s.Argv[0])
				if len(s.Argv) > 1 {
					what += " " + s.Argv[1]
				}
			}
			return fmt.Errorf("install: %s %s: %w", s.Kind, what, err)
		}
	}
	return nil
}

func apply(ctx context.Context, ops Ops, s Step) error {
	path := s.Path
	if user, ok := strings.CutPrefix(path, "~"); ok && path != "" {
		home, err := ops.Home(user)
		if err != nil {
			return err
		}
		path = home
	}
	switch s.Kind {
	case KindRun:
		return ops.Run(ctx, s.Argv)
	case KindRunTolerant:
		_ = ops.Run(ctx, s.Argv) // the label may not be loaded
		return nil
	case KindUntilGone:
		return untilGone(ctx, ops, s.Argv)
	case KindMkdir:
		return ops.Mkdir(path, s.Mode, s.Owner)
	case KindWrite:
		return ops.Write(path, []byte(s.Content), s.Mode, s.Owner)
	case KindCopy:
		return ops.Copy(s.Source, path, s.Mode, s.Owner)
	case KindChmod:
		return ops.Chmod(path, s.Mode, s.Owner)
	case KindRemove:
		return ops.Remove(path)
	case KindConfigAdd:
		return config.AddSessionUser(path, s.Name)
	case KindConfigRemove:
		return config.RemoveSessionUser(path, s.Name)
	}
	s.Path = path
	return ops.Windows(ctx, s)
}

// The wait of an until-gone step: at most unloadLooks looks, unloadPause apart, which is the
// time the executor of Windows gives a service to stop (stopService). Protocol constants of the
// executor, not product windows; the tests shorten the pause.
const unloadLooks = 300

var unloadPause = 100 * time.Millisecond

// errNotGone is the failure of an until-gone step whose command still succeeded at the last look.
var errNotGone = errors.New("the service did not unload")

// untilGone runs argv until it fails, which tells that what it looks up is gone: launchd knows
// the label of a service it unloads until the processes of the service have ended. Errors:
// errNotGone after unloadLooks looks, and the error of ctx when it ends first.
func untilGone(ctx context.Context, ops Ops, argv []string) error {
	for range unloadLooks {
		err := ops.Run(ctx, argv)
		if cerr := ctx.Err(); cerr != nil {
			return cerr
		}
		if err != nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(unloadPause):
		}
	}
	return errNotGone
}
