// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"context"
	"encoding/json"
	"math/rand/v2"
	"net/http"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// snapshotPath is the path of getSnapshot.
const snapshotPath = "/api/v1/sync/snapshot"

// snapshot is the answer of getSnapshot as the tests read it.
type snapshot struct {
	HeadSeq           int64  `json:"head_seq"`
	SyncEpoch         string `json:"sync_epoch"`
	PermissionVersion int64  `json:"permission_version"`
	Principal         struct {
		ID, Kind, Handle string
	} `json:"principal"`
	IsOwner bool `json:"is_owner"`
	Spaces  []struct {
		Space struct {
			ID string `json:"id"`
		} `json:"space"`
		Level string `json:"level"`
	} `json:"spaces"`
	Channels []struct {
		Channel struct {
			ID      string `json:"id"`
			SpaceID string `json:"space_id"`
		} `json:"channel"`
	} `json:"channels"`
	Sidebar []struct {
		SpaceID      string   `json:"space_id"`
		ChannelOrder []string `json:"channel_order"`
	} `json:"sidebar"`
	SidebarChannels []struct {
		ChannelID   string   `json:"channel_id"`
		ThreadOrder []string `json:"thread_order"`
		Version     int64    `json:"version"`
	} `json:"sidebar_channels"`
	UnreadThreads []struct {
		ThreadID       string `json:"thread_id"`
		ChannelID      string `json:"channel_id"`
		UnreadCount    int    `json:"unread_count"`
		MentionCount   int    `json:"mention_count"`
		LastMessageSeq int64  `json:"last_message_seq"`
		LastReadSeq    int64  `json:"last_read_seq"`
	} `json:"unread_threads"`
	ActiveRuns []json.RawMessage `json:"active_runs"`
	Agents     []struct {
		AgentID string `json:"agent_id"`
		State   string `json:"state"`
	} `json:"agents"`
	Settings []struct {
		Key string `json:"key"`
	} `json:"settings"`
	Drafts []struct {
		ID        string `json:"id"`
		ChannelID string `json:"channel_id"`
		ThreadID  string `json:"thread_id"`
	} `json:"drafts"`
	Features []struct {
		Key string `json:"key"`
	} `json:"features"`
}

// snap reads the snapshot of token name; the harness validated it against the schema Snapshot.
func (e *syncEnv) snap(name string) (snapshot, []byte) {
	e.t.Helper()
	r := e.Do(name, http.MethodGet, snapshotPath, "")
	e.Want(r, http.StatusOK, "")
	var s snapshot
	if err := json.Unmarshal(r.Body, &s); err != nil {
		e.t.Fatal(err)
	}
	return s, r.Body
}

// channelIDs returns the channel ids of a snapshot.
func (s snapshot) channelIDs() []string {
	var out []string
	for _, c := range s.Channels {
		out = append(out, c.Channel.ID)
	}
	return out
}

// TestSnapshotContents: a world with spaces, channels, threads, read states, mentions, drafts and
// settings; every part as in design note 9, the response valid against Snapshot with features.
func TestSnapshotContents(t *testing.T) {
	t.Parallel()
	e := newSyncEnv(t, 100000)
	forum := e.Channel(e.support, "forum", "ideas")
	helper := e.Agent("helper")
	e.SetOperator(e.supportChan, helper)
	e.Agent("stranger")
	for i := range 3 {
		e.Send("owner", e.supportMain, "plain "+strconv.Itoa(i), http.StatusCreated)
	}
	e.Want(e.Do("alex", http.MethodPut, readStatePath(e.supportMain), `{"last_read_seq":1}`), http.StatusOK, "")
	e.Send("owner", e.supportMain, "@alex please look", http.StatusCreated)
	e.Send("owner", e.marketingMain, "@alex not for you", http.StatusCreated)
	e.Want(e.Do("alex", http.MethodPut, threadDraftPath(e.supportMain), `{"content_md":"draft text"}`), http.StatusOK, "")
	e.Want(e.Do("alex", http.MethodPut, settingPath("appearance.theme"), `{"value":"dark"}`), http.StatusOK, "")
	e.Want(e.Do("alex", http.MethodPut, "/api/v1/me/sidebar/spaces/"+e.support.String(),
		`{"collapsed":true,"channel_order":["`+forum.String()+`","`+e.supportChan.String()+`"]}`), http.StatusOK, "")
	post := ids.New()
	e.Want(e.Do("alex", http.MethodPut, "/api/v1/me/sidebar/channels/"+forum.String(), `{"thread_order":["`+post.String()+`"]}`), http.StatusOK, "")
	// A row of Alex for a channel Alex cannot read (written as a fixture) stays out (M4-51).
	e.SQL(`INSERT INTO user_channel_states (user_id, channel_id) VALUES ($1, $2)`, e.alex.String(), e.marketingChan.String())

	s, _ := e.snap("alex")
	var epoch string
	if err := e.Owner.QueryRow(context.Background(), `SELECT sync_epoch::text FROM instance`).Scan(&epoch); err != nil {
		t.Fatal(err)
	}
	if s.HeadSeq != e.head() || s.SyncEpoch != epoch || s.PermissionVersion < 0 {
		t.Fatalf("head %d (want %d), epoch %s (want %s)", s.HeadSeq, e.head(), s.SyncEpoch, epoch)
	}
	if s.Principal.Handle != "alex" || s.Principal.Kind != "user" || s.IsOwner {
		t.Fatalf("principal %+v, owner %v", s.Principal, s.IsOwner)
	}
	if len(s.Spaces) != 1 || s.Spaces[0].Space.ID != e.support.String() || s.Spaces[0].Level != "read" {
		t.Fatalf("spaces %+v", s.Spaces)
	}
	if got := s.channelIDs(); len(got) != 2 || !slices.Contains(got, e.supportChan.String()) || !slices.Contains(got, forum.String()) {
		t.Fatalf("channels %v", got)
	}
	if len(s.Sidebar) != 1 || s.Sidebar[0].SpaceID != e.support.String() ||
		!slices.Equal(s.Sidebar[0].ChannelOrder, []string{forum.String(), e.supportChan.String()}) {
		t.Fatalf("sidebar %+v", s.Sidebar)
	}
	if len(s.SidebarChannels) != 1 || s.SidebarChannels[0].ChannelID != forum.String() ||
		!slices.Equal(s.SidebarChannels[0].ThreadOrder, []string{post.String()}) || s.SidebarChannels[0].Version != 1 {
		t.Fatalf("sidebar channels %+v", s.SidebarChannels)
	}
	if len(s.UnreadThreads) != 1 {
		t.Fatalf("unread threads %+v", s.UnreadThreads)
	}
	u := s.UnreadThreads[0]
	if u.ThreadID != e.supportMain.String() || u.ChannelID != e.supportChan.String() || u.LastMessageSeq != 4 || u.LastReadSeq != 1 ||
		u.UnreadCount != 3 || u.MentionCount != 1 {
		t.Fatalf("unread thread %+v", u)
	}
	if len(s.ActiveRuns) != 0 {
		t.Fatalf("active runs %s", s.ActiveRuns)
	}
	if len(s.Agents) != 1 || s.Agents[0].AgentID != helper.String() || s.Agents[0].State != "unknown" {
		t.Fatalf("agents %+v", s.Agents)
	}
	if len(s.Settings) != 1 || s.Settings[0].Key != "appearance.theme" {
		t.Fatalf("settings %+v", s.Settings)
	}
	if len(s.Drafts) != 1 || s.Drafts[0].ThreadID != e.supportMain.String() || s.Drafts[0].ChannelID != e.supportChan.String() {
		t.Fatalf("drafts %+v", s.Drafts)
	}
	if len(s.Features) == 0 {
		t.Fatal("no features")
	}
	// The owner sees both spaces and is owner.
	if o, _ := e.snap("owner"); !o.IsOwner || len(o.Spaces) != 2 {
		t.Fatalf("owner snapshot: owner %v, %d spaces", o.IsOwner, len(o.Spaces))
	}
}

// TestSnapshotHasNoContent: no member content_md anywhere in the response, although the thread
// has messages and the caller a draft.
func TestSnapshotHasNoContent(t *testing.T) {
	t.Parallel()
	e := newSyncEnv(t, 100000)
	e.Send("owner", e.supportMain, "@alex message text", http.StatusCreated)
	e.Want(e.Do("alex", http.MethodPut, threadDraftPath(e.supportMain), `{"content_md":"draft words"}`), http.StatusOK, "")
	_, raw := e.snap("alex")
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	var walk func(v any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, c := range x {
				if k == "content_md" {
					t.Fatalf("content_md in the snapshot: %s", raw)
				}
				walk(c)
			}
		case []any:
			for _, c := range x {
				walk(c)
			}
		}
	}
	walk(doc)
}

// TestSnapshotAgentHasEmptyPersonalParts: an agent gets empty sidebar, unread threads, settings and
// drafts, and itself as principal.
func TestSnapshotAgentHasEmptyPersonalParts(t *testing.T) {
	t.Parallel()
	e := newSyncEnv(t, 100000)
	helper := e.Agent("helper")
	e.SetOperator(e.supportChan, helper)
	e.Send("owner", e.supportMain, "@helper hello", http.StatusCreated)
	s, _ := e.snap("helper")
	if s.Principal.Kind != "agent" || s.Principal.ID != helper.String() || s.IsOwner {
		t.Fatalf("principal %+v", s.Principal)
	}
	if len(s.Sidebar) != 0 || len(s.UnreadThreads) != 0 || len(s.Settings) != 0 || len(s.Drafts) != 0 {
		t.Fatalf("personal parts of an agent: %+v", s)
	}
	if !slices.Contains(s.channelIDs(), e.supportChan.String()) {
		t.Fatalf("the operated channel is missing: %v", s.channelIDs())
	}
}

// TestSnapshotHeadReadFirst: with a concurrent writer, every change with seq <= head_seq is in the
// snapshot (here: every channel.created up to head_seq names a channel of the snapshot).
func TestSnapshotHeadReadFirst(t *testing.T) {
	t.Parallel()
	e := newSyncEnv(t, 100000)
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			r := e.Do("owner", http.MethodPost, "/api/v1/spaces/"+e.support.String()+"/channels",
				`{"kind":"text","name":"c`+strconv.Itoa(i)+`"}`, "Idempotency-Key", ids.New().String())
			if r.StatusCode != http.StatusCreated {
				t.Errorf("create channel: %d %s", r.StatusCode, r.Body)
				return
			}
		}
	}()
	for range 5 {
		time.Sleep(20 * time.Millisecond)
		s, _ := e.snap("owner")
		have := s.channelIDs()
		rows, err := e.Owner.Query(context.Background(),
			`SELECT resource_id::text FROM event_log WHERE type = 'channel.created' AND seq <= $1`, s.HeadSeq)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				t.Fatal(err)
			}
			if !slices.Contains(have, id) {
				t.Errorf("channel %s created at or before head %d is missing", id, s.HeadSeq)
			}
		}
		rows.Close()
	}
	close(stop)
	wg.Wait()
}

// TestAZP5SnapshotNoForeignChannel is the snapshot part of AZ-P5 (07 section 11.2): in random
// worlds no entry of spaces, channels, unread_threads, drafts or sidebar and no counter refers to
// a channel (or space) the person cannot read. Drafts, read states and sidebar rows written while
// a grant existed stay stored after it is revoked. The seed is logged; a fixed number of worlds
// runs in make test.
func TestAZP5SnapshotNoForeignChannel(t *testing.T) {
	t.Parallel()
	const worlds = 3
	seed := uint64(time.Now().UnixNano())
	t.Logf("seed %d", seed)
	for n := range worlds {
		azp5World(t, rand.New(rand.NewPCG(seed, uint64(n)))) //nolint:gosec // G404: reproducible test worlds from the logged seed, no secret.
	}
}

// azp5World builds one random world and checks the snapshots of its people.
func azp5World(t *testing.T, rnd *rand.Rand) {
	t.Helper()
	e := newSyncEnv(t, 100000)
	type channel struct{ id, space, main ids.ID }
	spaces := []ids.ID{e.support, e.marketing}
	chans := []channel{{e.supportChan, e.support, e.supportMain}, {e.marketingChan, e.marketing, e.marketingMain}}
	for i := range 1 + rnd.IntN(3) {
		sp := spaces[rnd.IntN(len(spaces))]
		id := e.Channel(sp, "text", "x"+strconv.Itoa(i))
		chans = append(chans, channel{id, sp, e.MainThread(id)})
	}
	people := map[string]ids.ID{"alex": e.alex, "sam": e.sam, "robin": e.robin}
	// Every person writes a draft, a read state and a sidebar row where it can, then loses some
	// grants: the rows stay stored.
	for name := range people {
		for _, c := range chans {
			e.Send("owner", c.main, "@"+name+" hi", http.StatusCreated)
		}
	}
	for name := range people {
		for _, c := range chans {
			if r := e.Do(name, http.MethodPut, threadDraftPath(c.main), `{"content_md":"d"}`); r.StatusCode == http.StatusOK {
				e.Want(e.Do(name, http.MethodPut, readStatePath(c.main), `{"last_read_seq":1}`), http.StatusOK, "")
			}
		}
		for _, sp := range spaces {
			e.Do(name, http.MethodPut, "/api/v1/me/sidebar/spaces/"+sp.String(), `{"collapsed":false}`)
		}
	}
	// Random revokes and random channel grants.
	for _, p := range people {
		if rnd.IntN(2) == 0 {
			e.SQL(`UPDATE grants SET revoked_at = now() WHERE subject_principal_id = $1 AND target_space_id = $2`, p.String(),
				spaces[rnd.IntN(len(spaces))].String())
		}
		if rnd.IntN(2) == 0 {
			c := chans[rnd.IntN(len(chans))]
			e.SQL(`INSERT INTO grants (id, subject_kind, subject_principal_id, target_kind, target_channel_id, level, capabilities, reason, granted_by)
				VALUES ($1, 'user', $2, 'channel', $3, 'read', '{}', 'fixture', $4)`, ids.New().String(), p.String(), c.id.String(), e.OwnerID.String())
		}
	}
	e.SQL(`UPDATE sequence_counters SET value = value + 1 WHERE name = 'permission_version'`)
	for name, p := range people {
		readable := map[string]bool{}
		visibleSpace := map[string]bool{}
		for _, c := range chans {
			n := e.Count(`SELECT count(*)::int FROM grants WHERE subject_principal_id = $1 AND revoked_at IS NULL
				AND (target_space_id = $2 OR target_channel_id = $3)`, p.String(), c.space.String(), c.id.String())
			if n > 0 {
				readable[c.id.String()], visibleSpace[c.space.String()] = true, true
			}
		}
		s, _ := e.snap(name)
		for _, sp := range s.Spaces {
			if !visibleSpace[sp.Space.ID] {
				t.Errorf("%s: foreign space %s", name, sp.Space.ID)
			}
		}
		for _, c := range s.Channels {
			if !readable[c.Channel.ID] {
				t.Errorf("%s: foreign channel %s", name, c.Channel.ID)
			}
		}
		for _, u := range s.UnreadThreads {
			if !readable[u.ChannelID] {
				t.Errorf("%s: unread counter of the foreign channel %s", name, u.ChannelID)
			}
		}
		for _, d := range s.Drafts {
			if !readable[d.ChannelID] {
				t.Errorf("%s: draft in the foreign channel %s", name, d.ChannelID)
			}
		}
		for _, sb := range s.Sidebar {
			if !visibleSpace[sb.SpaceID] {
				t.Errorf("%s: sidebar row of the foreign space %s", name, sb.SpaceID)
			}
		}
		if len(s.Channels) != len(readable) {
			t.Errorf("%s: %d channels, %d readable", name, len(s.Channels), len(readable))
		}
	}
}
