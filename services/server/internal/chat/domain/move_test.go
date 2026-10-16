// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/chat/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestCanMove checks the domain rule of thread.move (brief M1-62 note 1b) in its order: a main
// thread, the same channel, an archived target, a dm channel as source or target, then the
// operator rule with IS NOT DISTINCT FROM semantics (03 threads, R-DM-11, R-DM-12).
func TestCanMove(t *testing.T) {
	lotse, texter := ids.New(), ids.New()
	src := domain.MovePlace{ID: ids.New(), Kind: domain.ChannelText}
	dst := domain.MovePlace{ID: ids.New(), Kind: domain.ChannelText}
	thread := domain.Thread{ID: ids.New(), ChannelID: src.ID, Kind: domain.ThreadKindThread, OperatorAgentID: lotse}
	main := thread
	main.Kind, main.OperatorAgentID = domain.ThreadKindMain, ids.ID{}
	free := thread
	free.OperatorAgentID = ids.ID{}
	archived, dm, forum := dst, dst, dst
	archived.Archived, dm.Kind, forum.Kind = true, domain.ChannelDM, domain.ChannelForum
	dmSource := src
	dmSource.Kind = domain.ChannelDM
	// A target that inherits the default operator of another space (Texter) differs from Lotse.
	inherited := domain.EffectiveOperator(true, ids.ID{}, texter)
	cases := []struct {
		name     string
		th       domain.Thread
		from, to domain.MovePlace
		op       ids.ID
		code     errs.Code
		pointer  string
	}{
		{"main thread", main, src, dst, ids.ID{}, errs.CodeConflict, ""},
		{"main thread before same channel", main, src, src, ids.ID{}, errs.CodeConflict, ""},
		{"same channel", thread, src, src, lotse, errs.CodeValidationFailed, "/target_channel_id"},
		{"archived target", thread, src, archived, lotse, errs.CodeConflict, ""},
		{"archived target before operator", thread, src, archived, texter, errs.CodeConflict, ""},
		{"dm target", thread, src, dm, lotse, errs.CodeConflict, ""},
		{"dm source", thread, dmSource, dst, lotse, errs.CodeConflict, ""},
		{"equal operator", thread, src, dst, lotse, "", ""},
		{"into a forum with the equal operator", thread, src, forum, lotse, "", ""},
		{"different operator", thread, src, dst, texter, errs.CodeOperatorMismatch, ""},
		{"target without operator", thread, src, dst, ids.ID{}, errs.CodeOperatorMismatch, ""},
		{"thread without operator into an operated channel", free, src, dst, lotse, errs.CodeOperatorMismatch, ""},
		{"both without operator", free, src, dst, ids.ID{}, "", ""},
		{"target inherits another space default", thread, src, dst, inherited, errs.CodeOperatorMismatch, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := domain.CanMove(c.th, c.from, c.to, c.op)
			if c.code == "" {
				if err != nil {
					t.Fatalf("err %v", err)
				}
				return
			}
			e, ok := errs.As(err)
			if !ok || e.Code != c.code {
				t.Fatalf("err %#v, want %s", err, c.code)
			}
			if c.pointer != "" && (len(e.Fields) != 1 || e.Fields[0].Pointer != c.pointer || e.Fields[0].Code != domain.FieldSameChannel) {
				t.Fatalf("fields %#v, want %s %s", e.Fields, c.pointer, domain.FieldSameChannel)
			}
		})
	}
}
