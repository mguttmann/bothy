// SPDX-License-Identifier: Apache-2.0

package install

import (
	"context"
	"fmt"
	"io/fs"
	"strings"

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
			return fmt.Errorf("install: %s %s%s: %w", s.Kind, s.Path, s.Name, err)
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
