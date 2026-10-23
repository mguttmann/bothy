// SPDX-License-Identifier: Apache-2.0

package canary

import "net/http"

// imports drives the eleven import operations and setChannelReadOnly of M8-03 as the owner: it
// lists the sources, creates one whose mapping creates a space and a text channel with canary
// names of class content, reads it, starts it, applies one batch with the space and the channel,
// lists its items, stores a cursor, hands the lead of the channel to Bothy, pauses and completes
// it, and cancels a second source. The channel of the completed import is then switched read-only
// and back with If-Match.
func (sc *Scenario) imports() {
	t := sc.T
	t.Helper()
	o := sc.W.Owner
	spaceName := sc.Reg.New(ClassContent, "", "imported space name")
	channelName := sc.Reg.New(ClassContent, "", "imported channel name")
	create := func(name string) map[string]any { return map[string]any{"create": map[string]any{"name": name}} }
	source := map[string]any{"kind": "discord", "name": "Old community server", "config": map[string]any{
		"version": 1, "guild_id": "100",
		"defaults": map[string]any{"unmapped_human": "placeholder", "unmapped_bot": "skip", "uncategorized_space": "skip"},
		"spaces":   []any{map[string]any{"discord_category": "200", "target": create(spaceName)}},
		"channels": []any{map[string]any{"discord_channel": "300", "target": create(channelName)}},
		"users":    []any{},
	}}
	sc.get(o, "/imports", ok...)
	id := jsonText(obj(sc.post(o, "/imports", source, http.StatusCreated))["id"])
	if id == "" {
		t.Errorf("imports: createImport answered no id")
		return
	}
	path := "/imports/" + id
	sc.get(o, path, ok...)
	sc.post(o, path+"/start", nil, http.StatusAccepted)
	batch := obj(sc.post(o, path+"/batches", map[string]any{"items": []any{
		map[string]any{"origin_key": "space:200", "item_kind": "space", "data": map[string]any{"name": spaceName, "sort_key": "a0"}},
		map[string]any{"origin_key": "channel:300", "item_kind": "channel", "data": map[string]any{
			"space_origin_key": "space:200", "name": channelName, "kind": "text", "sort_key": "a0"}},
	}}, ok...))
	channel := ""
	if items, isList := batch["items"].([]any); isList && len(items) == 2 {
		if it, isObj := items[1].(map[string]any); isObj {
			channel = jsonText(it["target_id"])
		}
	}
	sc.get(o, path+"/items", ok...)
	sc.put(o, path+"/cursor", map[string]any{"cursor": map[string]any{"300": "400"}, "stats": map[string]any{"messages": 0}}, ok...)
	sc.put(o, path+"/leads", map[string]any{"source_channel": "300", "lead": "bothy"}, ok...)
	sc.post(o, path+"/pause", nil, ok...)
	sc.post(o, path+"/complete", nil, ok...)
	other := jsonText(obj(sc.post(o, "/imports", source, http.StatusCreated))["id"])
	sc.post(o, "/imports/"+other+"/cancel", nil, ok...)
	if channel == "" {
		t.Errorf("imports: applyImportBatch answered no target of the channel")
		return
	}
	readOnly := "/channels/" + channel + "/read-only"
	sc.matched(o, "/channels/"+channel, req{method: http.MethodPut, path: readOnly,
		body: map[string]any{"read_only": true, "reason": "archive copy"}}, http.StatusOK)
	sc.matched(o, "/channels/"+channel, req{method: http.MethodPut, path: readOnly,
		body: map[string]any{"read_only": false, "reason": "open again"}}, http.StatusOK)
}
