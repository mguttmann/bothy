package app_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

func TestDebugAgentInvocationEvent(t *testing.T) {
	w := newWorld(t)
	r := authorize.ResourceFromEvent("run", ids.New(), w.support, w.allgemein, w.mainThread[w.allgemein])
	r.Event.AudienceID = w.lotse
	d, err := w.az.Authorize(authorize.Filter(context.Background()), w.agent(w.lotse), authorize.ActionAgentEventsConsume, r)
	t.Logf("%+v %v", d, err)
}
