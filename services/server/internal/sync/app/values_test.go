// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	mw "github.com/OWNER/bothy/services/server/test/messagingworld"
)

// TestSettings_ValuesFor (M3-18): the stored values of the asked keys of many people in one read,
// without authorize and without defaults; people and keys without a row are absent.
func TestSettings_ValuesFor(t *testing.T) {
	t.Parallel()
	w := mw.NewPersonal(t, noSubscriber)
	alex, sam := w.Person("alex", false), w.Person("sam", false)
	robin := w.Person("robin", false)
	for _, s := range []struct {
		name, key, value string
	}{{"alex", "notify.default_level", `"all"`}, {"alex", "appearance.theme", `"dark"`}, {"sam", "notify.default_level", `"mute"`}} {
		ctx := actor.WithActor(context.Background(), w.Actor(s.name))
		if _, err := w.Personal.Settings().Put(ctx, s.key, json.RawMessage(s.value)); err != nil {
			t.Fatal(err)
		}
	}
	got, err := w.Personal.Settings().ValuesFor(context.Background(), []ids.ID{alex, sam, robin, ids.New()}, []string{"notify.default_level"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || string(got[alex]["notify.default_level"]) != `"all"` || string(got[sam]["notify.default_level"]) != `"mute"` ||
		len(got[alex]) != 1 {
		t.Fatalf("values %v", got)
	}
	if empty, err := w.Personal.Settings().ValuesFor(context.Background(), nil, []string{"notify.default_level"}); err != nil || len(empty) != 0 {
		t.Fatalf("no people %v, %v", empty, err)
	}
}
