// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// BenchmarkHubDispatch delivers one event of a channel to many connections that see it and
// drains their queues; the gate on the value is the load test of M1-72.
func BenchmarkHubDispatch(b *testing.B) {
	w := newHubWorld()
	support := ids.New()
	allgemein := w.channel(support)
	h := newHub(b, w, hubCfg)
	var conns []*qconn
	for range 1000 {
		a := person()
		w.grantChannel(a.ID, allgemein)
		qc := connect(b, h, a, 4, 0, 0)
		conns = append(conns, qc)
	}
	ev := msg(1, support, allgemein)
	b.ResetTimer()
	for i := range b.N {
		ev.Seq = int64(i + 1)
		h.Dispatch(context.Background(), ev)
		b.StopTimer()
		for _, qc := range conns {
			qc.mu.Lock()
			qc.q.Clear()
			qc.mu.Unlock()
		}
		b.StartTimer()
	}
}
