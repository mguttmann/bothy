// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/instance/adapters/httpapi"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

func TestAdmin_ListFeatureFlagsConverts(t *testing.T) {
	t.Parallel()
	by, sam := ids.New(), ids.New()
	at := time.Date(2026, 10, 13, 9, 0, 0, 123456789, time.UTC)
	f := &fakeAdmin{flags: []instance.FeatureFlag{
		{Key: "dm.enabled", Kind: instance.FlagKindOption, Description: "Direct messages", Module: "chat"},
		{Key: "chat.box", Kind: instance.FlagKindRelease, Description: "Box", Module: "chat", RemoveBy: "1.4.0", Enabled: true, Version: 2,
			UpdatedBy: by, UpdatedAt: at, Rollout: instance.FlagRollout{UserIDs: []ids.ID{sam}}},
		{Key: "aa.old", Orphaned: true, Enabled: true, Version: 1},
	}}
	r, err := httpapi.NewAdmin(f).ListFeatureFlags(context.Background(), rest.ListFeatureFlagsRequestObject{})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(r.(rest.ListFeatureFlags200JSONResponse).Body)
	var got struct{ Items []map[string]any }
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	def := got.Items[0]
	if def["kind"] != "option" || def["default"] != false || def["module"] != "chat" || def["orphaned"] != false || def["version"] != float64(0) ||
		def["remove_by"] != nil || def["updated_at"] != nil || def["updated_by"] != nil || len(def["rollout"].(map[string]any)) != 0 {
		t.Errorf("default %v", def)
	}
	rel := got.Items[1]
	if rel["remove_by"] != "1.4.0" || rel["updated_by"] != by.String() || rel["updated_at"] != "2026-10-13T09:00:00.123Z" ||
		!slices.Equal(rel["rollout"].(map[string]any)["user_ids"].([]any), []any{sam.String()}) {
		t.Errorf("release %v", rel)
	}
	orphan := got.Items[2]
	for _, k := range []string{"kind", "default", "module", "description", "remove_by"} {
		if _, ok := orphan[k]; ok {
			t.Errorf("orphaned row has %s: %v", k, orphan)
		}
	}
	f.err = errors.New("down")
	if _, err := httpapi.NewAdmin(f).ListFeatureFlags(context.Background(), rest.ListFeatureFlagsRequestObject{}); !errors.Is(err, f.err) {
		t.Error("list error lost")
	}
}

func TestAdmin_PutFeatureFlagConverts(t *testing.T) {
	t.Parallel()
	g := ids.New()
	f := &fakeAdmin{flag: instance.FeatureFlag{Key: "dm.enabled", Kind: instance.FlagKindOption, Description: "x", Module: "chat", Enabled: true, Version: 3}}
	h := httpapi.NewAdmin(f)
	tag := `"2"`
	groups := []rest.Uuid{rest.Uuid(g)}
	r, err := h.PutFeatureFlag(context.Background(), rest.PutFeatureFlagRequestObject{
		Key: "dm.enabled", Params: rest.PutFeatureFlagParams{IfMatch: &tag},
		Body: &rest.FeatureFlagPut{Enabled: true, Reason: "pilot", Rollout: &rest.FlagRollout{GroupIds: &groups}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := *r.(rest.PutFeatureFlag200JSONResponse).Headers.ETag; got != `"3"` {
		t.Errorf("ETag %q", got)
	}
	want := instance.PutFlagCmd{Key: "dm.enabled", Enabled: true, Reason: "pilot", IfMatch: instance.IfMatch{Present: true, Valid: true, Version: 2},
		Rollout: instance.FlagRollout{GroupIDs: []ids.ID{g}}}
	if f.putFlag.Key != want.Key || f.putFlag.IfMatch != want.IfMatch || f.putFlag.Reason != "pilot" || !slices.Equal(f.putFlag.Rollout.GroupIDs, want.Rollout.GroupIDs) ||
		f.putFlag.Rollout.UserIDs != nil {
		t.Errorf("command %+v", f.putFlag)
	}
	f.err = errs.VersionConflict(instance.FeatureFlag{Key: "dm.enabled", Version: 5})
	_, err = h.PutFeatureFlag(context.Background(), rest.PutFeatureFlagRequestObject{Key: "dm.enabled", Body: &rest.FeatureFlagPut{Reason: "x"}})
	if e, ok := errs.As(err); !ok || e.Current.(rest.FeatureFlag).Version != 5 {
		t.Errorf("current not in API form: %v", err)
	}
}
