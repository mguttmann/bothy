// SPDX-License-Identifier: Apache-2.0

package app

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/chat/domain"
	"github.com/OWNER/bothy/services/server/internal/chat/domain/title"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestTitleByClassification (brief M2-18 note 6a): a person is ByPerson; the
// agent equal to the thread's operator_agent_id is ByOperatorAgent; another agent, an app and a
// thread without operator give ByOther.
func TestTitleByClassification(t *testing.T) {
	t.Parallel()
	lotse, texter, person := ids.New(), ids.New(), ids.New()
	th := domain.Thread{OperatorAgentID: lotse}
	for _, c := range []struct {
		a    actor.Actor
		th   domain.Thread
		want title.By
	}{
		{actor.Actor{ID: person, Kind: actor.KindUser}, th, title.ByPerson},
		{actor.Actor{ID: lotse, Kind: actor.KindAgent}, th, title.ByOperatorAgent},
		{actor.Actor{ID: texter, Kind: actor.KindAgent}, th, title.ByOther},
		{actor.Actor{ID: lotse, Kind: actor.KindApp}, th, title.ByOther},
		{actor.Actor{ID: lotse, Kind: actor.KindAgent}, domain.Thread{}, title.ByOther},
	} {
		if got := titleBy(c.a, c.th); got != c.want {
			t.Errorf("%+v: %v, want %v", c.a, got, c.want)
		}
	}
}
