// SPDX-License-Identifier: Apache-2.0

package apns_test

import (
	"log/slog"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/notify/adapters/apns/apnstest"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
)

// answerRow is one row of the answer table of 13 7.1 and Design note 6 of brief M3-20.
type answerRow struct {
	name   string
	resp   apnstest.Response
	class  domain.ResultClass
	reason string
	since  bool
	retry  time.Duration
	level  slog.Level
	logged bool
}

// tableTimestamp is the scripted timestamp of the 410 rows.
var tableTimestamp = time.Date(2026, 10, 20, 8, 0, 0, 123_000_000, time.UTC)

// answerRows returns the rows of the table, the unlisted ones included.
func answerRows() []answerRow {
	gone := func(reason string) apnstest.Response {
		return apnstest.Response{Status: 410, Reason: reason, Timestamp: tableTimestamp}
	}
	errRow := func(name string, status int, reason string, class domain.ResultClass) answerRow {
		return answerRow{name: name, resp: apnstest.Response{Status: status, Reason: reason}, class: class, level: slog.LevelError, logged: true}
	}
	rows := []answerRow{
		{name: "200", resp: apnstest.Response{}, class: domain.ResultAccepted},
		{name: "410 Unregistered", resp: gone("Unregistered"), class: domain.ResultDisable, reason: domain.DisableTokenInvalid, since: true},
		{name: "410 ExpiredToken", resp: gone("ExpiredToken"), class: domain.ResultDisable, reason: domain.DisableTokenInvalid, since: true},
		{name: "410 Other", resp: gone("Other"), class: domain.ResultDisable, reason: domain.DisableTokenInvalid, since: true},
		{name: "400 BadDeviceToken", resp: apnstest.Response{Status: 400, Reason: "BadDeviceToken"}, class: domain.ResultDisable,
			reason: domain.DisableTokenInvalid, level: slog.LevelWarn, logged: true},
		{name: "400 DeviceTokenNotForTopic", resp: apnstest.Response{Status: 400, Reason: "DeviceTokenNotForTopic"}, class: domain.ResultDisable,
			reason: domain.DisableTopicMismatch},
		errRow("429 TooManyProviderTokenUpdates", 429, "TooManyProviderTokenUpdates", domain.ResultProgramError),
		{name: "429 TooManyRequests", resp: apnstest.Response{Status: 429, Reason: "TooManyRequests"}, class: domain.ResultThrottled},
		{name: "429 Other", resp: apnstest.Response{Status: 429, Reason: "Other"}, class: domain.ResultThrottled},
		errRow("413 PayloadTooLarge", 413, "PayloadTooLarge", domain.ResultProgramError),
		errRow("400 BadTopic", 400, "BadTopic", domain.ResultProgramError),
		errRow("404 BadPath", 404, "BadPath", domain.ResultProgramError),
		errRow("405 MethodNotAllowed", 405, "MethodNotAllowed", domain.ResultProgramError),
		errRow("418 no body", 418, "", domain.ResultProgramError),
		{name: "500", resp: apnstest.Response{Status: 500, Reason: "InternalServerError"}, class: domain.ResultRetry, retry: retry5xx},
		{name: "503", resp: apnstest.Response{Status: 503, Reason: "ServiceUnavailable"}, class: domain.ResultRetry, retry: retry5xx},
		{name: "502", resp: apnstest.Response{Status: 502}, class: domain.ResultRetry, retry: retry5xx},
		errRow("204", 204, "", domain.ResultProgramError),
		errRow("301", 301, "", domain.ResultProgramError),
	}
	for _, r := range []string{"InvalidProviderToken", "MissingProviderToken", "BadEnvironmentKeyIdInToken", "UnrelatedKeyIdInToken", "Forbidden", "Other"} {
		rows = append(rows, errRow("403 "+r, 403, r, domain.ResultConfigError))
	}
	return rows
}

func TestAnswerTable(t *testing.T) {
	t.Parallel()
	for _, row := range answerRows() {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			w := newWorld(t)
			w.prod.Script(w.phone.Token, row.resp)
			res := w.send(w.phone, w.hint())
			if res.Class != row.class || res.DisableReason != row.reason || res.RetryAfter != row.retry {
				t.Fatalf("result %+v, want class %s reason %q retry %s", res, row.class, row.reason, row.retry)
			}
			if row.since != !res.InvalidSince.IsZero() || (row.since && !res.InvalidSince.Equal(tableTimestamp)) {
				t.Fatalf("InvalidSince %s", res.InvalidSince)
			}
			recs := w.log.records()
			if !row.logged {
				if len(recs) != 0 {
					t.Fatalf("records %+v, want none", recs)
				}
				return
			}
			if len(recs) != 1 || recs[0].Level != row.level {
				t.Fatalf("records %+v, want one at %s", recs, row.level)
			}
		})
	}
	t.Run("Fake closed", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		w.prod.Close()
		if _, err := w.a.Send(t.Context(), w.phone, w.hint()); err == nil {
			t.Fatal("no Go error against a closed Fake")
		}
	})
}
