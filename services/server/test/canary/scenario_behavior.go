// SPDX-License-Identifier: Apache-2.0

package canary

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// behavior drives the four operations of channel behaviour and agent configuration (M2-02) with a
// canary prompt and a canary skill reference of class manage_only on Support/Allgemein: the
// owner sets them, the operator Lotse lists and reads them and pages listEvents (a foreign record
// of Lotse), Alex (reads Support, no manage) is refused on read and write, and the owner sends a
// stale PUT (412 with current), a PUT whose prompt exceeds channel.prompt_max_bytes (400) and a
// switch-only PUT. Sam reads Support/Allgemein too, so his gateway frames and pages carry the
// settings events of the channel.
func (sc *Scenario) behavior() {
	t := sc.T
	t.Helper()
	o, alex, lotse := sc.W.Owner, sc.W.People["Alex"], sc.W.Agents["Lotse"]
	allgemein := sc.W.Channels["Support/Allgemein"]
	path := "/channels/" + allgemein + "/behavior"

	put := func(prompt string, skills []string, mention bool) map[string]any {
		return map[string]any{
			"require_mention": mention, "thread_require_mention": false, "free_response": false, "auto_thread": true,
			"allow_mentioned_agents": false, "history_backfill_limit": 20, "prompt": prompt, "skills": skills,
		}
	}
	prompt := "Answer politely. " + sc.Reg.New(ClassManageOnly, allgemein, "channel prompt")
	skill := sc.Reg.New(ClassManageOnly, allgemein, "skill reference")
	sc.matched(o, path, req{method: http.MethodPut, path: path, body: put(prompt, []string{"triage", skill}, true)}, http.StatusOK)
	sc.get(o, path, ok...)

	sc.get(lotse, "/agents/me/channels", ok...)
	sc.get(lotse, "/agents/me/channels/"+allgemein+"/config", ok...)

	sc.get(alex, path, http.StatusForbidden)
	sc.call(alex, req{method: http.MethodPut, path: path, header: ifMatch(1),
		body: put(sc.Reg.New(ClassManageOnly, allgemein, "channel prompt (403)"), []string{sc.Reg.New(ClassManageOnly, allgemein, "skill reference (403)")}, true)},
		http.StatusForbidden)

	cur := sc.get(o, path, ok...)
	version := jsonText(obj(cur)["settings_version"])
	stale, _ := strconv.ParseInt(version, 10, 64)
	sc.call(o, req{method: http.MethodPut, path: path, header: ifMatch(stale + 5),
		body: put(sc.Reg.New(ClassManageOnly, allgemein, "channel prompt (412)"), []string{skill}, true)}, http.StatusPreconditionFailed)
	limit, err := strconv.Atoi(sc.Server.ConfigValue("channel.prompt_max_bytes"))
	if err != nil {
		t.Errorf("channel.prompt_max_bytes: %v", err)
	}
	long := sc.Reg.New(ClassManageOnly, allgemein, "channel prompt (400)") + " " + strings.Repeat("x", limit)
	sc.call(o, req{method: http.MethodPut, path: path, header: ifMatch(stale), body: put(long, []string{skill}, true)}, http.StatusBadRequest)
	sc.call(o, req{method: http.MethodPut, path: path, header: ifMatch(stale), body: put(prompt, []string{"triage", skill}, false)}, ok...)

	sc.EventPages(lotse, func(page int, body []byte, _ []map[string]any) {
		sc.Sinks.AddJSON(SinkForeign, fmt.Sprintf("Lotse listEvents page %d", page), "Lotse", "", body)
	})
}
