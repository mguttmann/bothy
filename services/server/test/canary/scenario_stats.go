// SPDX-License-Identifier: Apache-2.0

package canary

import (
	"net/http"
	"net/url"
	"time"
)

// statistics drives the five statistics operations of M6-05 after the runs and costs of the world
// exist: the owner reads each of them instance-wide over the last two days of the server clock,
// and Alex reads the usage of the space Support, which answers 200 or 403 depending on Alex's
// rights. The answers hold only counts and sums; the scan checks that no canary reaches a sink.
// Then the owner reads the views of M6-14: the timeline of the last hour on full minutes, the
// traffic light and the backend view of the same two days.
func (sc *Scenario) statistics() {
	sc.T.Helper()
	owner, alex := sc.W.Owner, sc.W.People["Alex"]
	now := sc.Server.Clock().Now()
	rng := "?from=" + url.QueryEscape(timestamp(now.Add(-48*time.Hour))) + "&to=" + url.QueryEscape(timestamp(now))
	for _, p := range []string{"/stats/usage", "/stats/runs", "/stats/tool-calls", "/stats/errors", "/stats/costs"} {
		sc.get(owner, p+rng, ok...)
	}
	sc.get(alex, "/stats/usage"+rng+"&space_id="+sc.W.Spaces["Support"], http.StatusOK, http.StatusForbidden)
	to := now.UTC().Truncate(time.Minute)
	sc.get(owner, "/stats/timeline?from="+url.QueryEscape(timestamp(to.Add(-time.Hour)))+"&to="+url.QueryEscape(timestamp(to)), ok...)
	sc.get(owner, "/stats/slo", ok...)
	sc.get(owner, "/stats/backends"+rng, ok...)
}
