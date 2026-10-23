// SPDX-License-Identifier: Apache-2.0

package fcm

import (
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/notify/domain"
)

// errBody returns an FCM error body with the given status, errorCode and token violation.
func errBody(code int, status, errorCode string, tokenField bool) []byte {
	details := ""
	if errorCode != "" {
		details = `{"@type":"type.googleapis.com/google.firebase.fcm.v1.FcmError","errorCode":"` + errorCode + `"}`
	}
	if tokenField {
		if details != "" {
			details += ","
		}
		details += `{"@type":"type.googleapis.com/google.rpc.BadRequest","fieldViolations":[{"field":"message.token","description":"x"}]}`
	}
	return []byte(`{"error":{"code":` + strconv.Itoa(code) + `,"message":"canary","status":"` + status + `","details":[` + details + `]}}`)
}

func TestClassifyTable(t *testing.T) {
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name    string
		status  int
		body    []byte
		header  http.Header
		class   domain.ResultClass
		reason  string
		after   time.Duration
		renew   bool
		pause   bool
		log     logLevel
		errType string
	}{
		{"200", 200, []byte(`{"name":"projects/x/messages/1"}`), nil, domain.ResultAccepted, "", 0, false, false, logNone, ""},
		{"404 UNREGISTERED", 404, errBody(404, "NOT_FOUND", "UNREGISTERED", false), nil, domain.ResultDisable, domain.DisableTokenInvalid, 0, false, false, logNone, ""},
		{"400 token violation", 400, errBody(400, "INVALID_ARGUMENT", "INVALID_ARGUMENT", true), nil, domain.ResultDisable, domain.DisableTokenInvalid, 0, false, false, logNone, ""},
		// Google's example of an invalid token: only the FcmError detail, no BadRequest.
		{"400 FcmError INVALID_ARGUMENT without violation", 400, errBody(400, "INVALID_ARGUMENT", "INVALID_ARGUMENT", false), nil, domain.ResultDisable, domain.DisableTokenInvalid, 0, false, false, logNone, ""},
		// Google's example of a bad field value: a BadRequest violation on another field.
		{"400 other field violation", 400, []byte(`{"error":{"code":400,"message":"canary","status":"INVALID_ARGUMENT","details":[{"@type":"type.googleapis.com/google.rpc.BadRequest","fieldViolations":[{"field":"message.data[0].value","description":"x"}]}]}}`), nil, domain.ResultProgramError, "", 0, false, false, logErrorProgram, "other"},
		{"400 FcmError with other field violation", 400, []byte(`{"error":{"code":400,"message":"canary","status":"INVALID_ARGUMENT","details":[{"@type":"type.googleapis.com/google.firebase.fcm.v1.FcmError","errorCode":"INVALID_ARGUMENT"},{"@type":"type.googleapis.com/google.rpc.BadRequest","fieldViolations":[{"field":"message.android.ttl","description":"x"}]}]}}`), nil, domain.ResultProgramError, "", 0, false, false, logErrorProgram, "INVALID_ARGUMENT"},
		{"400 without FcmError", 400, errBody(400, "INVALID_ARGUMENT", "", false), nil, domain.ResultProgramError, "", 0, false, false, logErrorProgram, "other"},
		{"403 SENDER_ID_MISMATCH", 403, errBody(403, "PERMISSION_DENIED", "SENDER_ID_MISMATCH", false), nil, domain.ResultDisable, domain.DisableSenderMismatch, 0, false, false, logErrorSender, "SENDER_ID_MISMATCH"},
		{"401 THIRD_PARTY_AUTH_ERROR", 401, errBody(401, "UNAUTHENTICATED", "THIRD_PARTY_AUTH_ERROR", false), nil, domain.ResultConfigError, "", 0, false, false, logErrorConfig, "THIRD_PARTY_AUTH_ERROR"},
		{"401 UNAUTHENTICATED", 401, errBody(401, "UNAUTHENTICATED", "", false), nil, "", "", 0, true, false, logNone, "UNAUTHENTICATED"},
		{"403 PERMISSION_DENIED", 403, errBody(403, "PERMISSION_DENIED", "", false), nil, domain.ResultConfigError, "", 0, false, true, logNone, "PERMISSION_DENIED"},
		{"404 NOT_FOUND", 404, errBody(404, "NOT_FOUND", "", false), nil, domain.ResultConfigError, "", 0, false, true, logNone, "NOT_FOUND"},
		{"429 QUOTA_EXCEEDED", 429, errBody(429, "RESOURCE_EXHAUSTED", "QUOTA_EXCEEDED", false), nil, domain.ResultThrottled, "", time.Minute, false, false, logNone, ""},
		{"429 other", 429, nil, nil, domain.ResultThrottled, "", time.Minute, false, false, logNone, ""},
		{"503 UNAVAILABLE", 503, errBody(503, "UNAVAILABLE", "UNAVAILABLE", false), http.Header{"Retry-After": {"120"}}, domain.ResultRetry, "", 2 * time.Minute, false, false, logNone, ""},
		{"500 INTERNAL", 500, errBody(500, "INTERNAL", "INTERNAL", false), nil, domain.ResultRetry, "", 0, false, false, logNone, ""},
		{"502", 502, []byte("<html>"), nil, domain.ResultRetry, "", 0, false, false, logNone, ""},
		{"409", 409, nil, nil, domain.ResultProgramError, "", 0, false, false, logErrorProgram, "other"},
		{"204", 204, nil, nil, domain.ResultProgramError, "", 0, false, false, logErrorProgram, "other"},
		{"301", 301, nil, nil, domain.ResultProgramError, "", 0, false, false, logErrorProgram, "other"},
		{"broken body 404", 404, []byte("{"), nil, domain.ResultConfigError, "", 0, false, true, logNone, "other"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v := classify(c.status, c.body, c.header, now)
			if v.result.Class != c.class || v.result.DisableReason != c.reason || v.result.RetryAfter != c.after {
				t.Fatalf("result = %+v, want class %q reason %q after %v", v.result, c.class, c.reason, c.after)
			}
			if v.renew != c.renew || v.pause != c.pause || v.log != c.log {
				t.Fatalf("renew %v pause %v log %v, want %v %v %v", v.renew, v.pause, v.log, c.renew, c.pause, c.log)
			}
			if v.errType != c.errType {
				t.Fatalf("errType = %q, want %q", v.errType, c.errType)
			}
		})
	}
}

func TestRetryAfterParse(t *testing.T) {
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		value string
		want  time.Duration
	}{
		{"120", 2 * time.Minute},
		{now.Add(90 * time.Second).Format(http.TimeFormat), 90 * time.Second},
		{now.Add(-time.Minute).Format(http.TimeFormat), 0},
		{"-5", 0},
		{"soon", 0},
		{"", 0},
		{"30", 30 * time.Second},
	}
	for _, c := range cases {
		if got := retryAfter(http.Header{"Retry-After": {c.value}}, now); got != c.want {
			t.Errorf("retryAfter(%q) = %v, want %v", c.value, got, c.want)
		}
	}
	if got := retryAfter(nil, now); got != 0 {
		t.Errorf("retryAfter(nil) = %v", got)
	}
}
