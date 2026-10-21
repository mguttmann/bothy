// SPDX-License-Identifier: Apache-2.0

package fcm

import (
	"encoding/json"
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/internal/notify/domain"
)

// The FCM error codes of 13 7.2 [Q16] (errorCode of the detail FcmError).
const (
	codeUnregistered        = "UNREGISTERED"
	codeInvalidArgument     = "INVALID_ARGUMENT"
	codeSenderIDMismatch    = "SENDER_ID_MISMATCH"
	codeThirdPartyAuthError = "THIRD_PARTY_AUTH_ERROR"
	codeQuotaExceeded       = "QUOTA_EXCEEDED"
	codeUnavailable         = "UNAVAILABLE"
	codeInternal            = "INTERNAL"
)

// The RPC status values of error.status that error.type may carry (Design note 7 a of brief
// M3-21).
const (
	statusUnauthenticated  = "UNAUTHENTICATED"
	statusPermissionDenied = "PERMISSION_DENIED"
	statusNotFound         = "NOT_FOUND"
)

// The @type values of the details of an FCM error body [Q16].
const (
	typeFcmError   = "type.googleapis.com/google.firebase.fcm.v1.FcmError"
	typeBadRequest = "type.googleapis.com/google.rpc.BadRequest"
	fieldToken     = "message.token"
)

// errorTypeOther is error.type of an answer outside the lists, so an answer of Google never puts
// free text into a log record.
const errorTypeOther = "other"

// tableCodes are the FCM error codes that error.type may carry.
var tableCodes = []string{
	codeUnregistered, codeInvalidArgument, codeSenderIDMismatch, codeThirdPartyAuthError,
	codeQuotaExceeded, codeUnavailable, codeInternal,
}

// tableStatuses are the error.status values that error.type may carry.
var tableStatuses = []string{statusUnauthenticated, statusPermissionDenied, statusNotFound}

// quotaMinWait is the shortest wait after 429 QUOTA_EXCEEDED: Google documents a first wait of
// at least one minute (13 7.2 [Q16]). It is a provider rule, not an operator value.
const quotaMinWait = time.Minute

// logLevel says which record an answer writes (Design note 7 of brief M3-21).
type logLevel int

const (
	logNone logLevel = iota
	logErrorConfig
	logErrorProgram
	logErrorSender
)

// verdict is the consequence of one FCM answer.
type verdict struct {
	// result is the SendResult for the job of M3-19.
	result domain.SendResult
	// renew asks for the one access token renewal and retry of 401 UNAUTHENTICATED.
	renew bool
	// pause starts the pause of the adapter (a credential error; settle logs it once).
	pause bool
	// log is the record the answer writes itself; errType its error.type, also the error.type of
	// a pause.
	log     logLevel
	errType string
}

// errorBody is the part of an FCM error answer that classify reads. error.message is not
// decoded at all, because Google may echo request values in it.
type errorBody struct {
	Error struct {
		Status  string `json:"status"`
		Details []struct {
			Type            string `json:"@type"`
			ErrorCode       string `json:"errorCode"`
			FieldViolations []struct {
				Field string `json:"field"`
			} `json:"fieldViolations"`
		} `json:"details"`
	} `json:"error"`
}

// parsed is what classify takes from a body: the FCM error code, error.status and whether a
// BadRequest violation names message.token. A missing or broken body gives the zero value.
type parsed struct {
	code, status string
	tokenField   bool
}

// parseBody reads the error body; a body that does not decode leaves the status alone to decide.
func parseBody(body []byte) parsed {
	var b errorBody
	if len(body) == 0 || json.Unmarshal(body, &b) != nil {
		return parsed{}
	}
	p := parsed{status: b.Error.Status}
	for _, d := range b.Error.Details {
		switch d.Type {
		case typeFcmError:
			if p.code == "" {
				p.code = d.ErrorCode
			}
		case typeBadRequest:
			for _, v := range d.FieldViolations {
				if v.Field == fieldToken {
					p.tokenField = true
				}
			}
		}
	}
	return p
}

// errorType returns the FCM error code when it is one of the table, else the status when it is
// one of UNAUTHENTICATED, PERMISSION_DENIED and NOT_FOUND, else "other".
func (p parsed) errorType() string {
	switch {
	case slices.Contains(tableCodes, p.code):
		return p.code
	case slices.Contains(tableStatuses, p.status):
		return p.status
	}
	return errorTypeOther
}

// classify maps one send answer onto the table of 13 7.2 with the unlisted rows of Design note 6
// of brief M3-21: the HTTP status decides the class, the FCM error code the detail, and an
// unlisted answer follows the nearest listed answer of the same status. header gives Retry-After,
// read against now. It is a pure function.
func classify(status int, body []byte, header http.Header, now time.Time) verdict {
	p := parseBody(body)
	program := verdict{result: domain.SendResult{Class: domain.ResultProgramError}, log: logErrorProgram, errType: p.errorType()}
	configPause := verdict{result: domain.SendResult{Class: domain.ResultConfigError}, pause: true, errType: p.errorType()}
	switch {
	case status == http.StatusOK:
		return verdict{result: domain.SendResult{Class: domain.ResultAccepted}}
	case status == http.StatusNotFound && p.code == codeUnregistered:
		return verdict{result: domain.SendResult{Class: domain.ResultDisable, DisableReason: domain.DisableTokenInvalid}}
	case status == http.StatusBadRequest && p.tokenField:
		return verdict{result: domain.SendResult{Class: domain.ResultDisable, DisableReason: domain.DisableTokenInvalid}}
	case status == http.StatusForbidden && p.code == codeSenderIDMismatch:
		return verdict{result: domain.SendResult{Class: domain.ResultDisable, DisableReason: domain.DisableSenderMismatch},
			log: logErrorSender, errType: codeSenderIDMismatch}
	case status == http.StatusUnauthorized && p.code == codeThirdPartyAuthError:
		// The answer concerns one registration's APNs credentials at Google: no pause.
		return verdict{result: domain.SendResult{Class: domain.ResultConfigError}, log: logErrorConfig, errType: codeThirdPartyAuthError}
	case status == http.StatusUnauthorized:
		return verdict{renew: true, errType: statusUnauthenticated}
	case status == http.StatusForbidden, status == http.StatusNotFound:
		return configPause
	case status == http.StatusTooManyRequests:
		return verdict{result: domain.SendResult{Class: domain.ResultThrottled, RetryAfter: max(retryAfter(header, now), quotaMinWait)}}
	case status >= 500 && status < 600:
		return verdict{result: domain.SendResult{Class: domain.ResultRetry, RetryAfter: retryAfter(header, now)}}
	}
	return program
}

// retryAfter reads Retry-After as delta seconds or as an HTTP date against now (RFC 9110
// 10.2.3); a negative, unparsable or past value counts as absent and gives zero.
func retryAfter(header http.Header, now time.Time) time.Duration {
	v := strings.TrimSpace(header.Get("Retry-After"))
	if v == "" {
		return 0
	}
	if n, err := strconv.ParseInt(v, 10, 64); err == nil {
		switch {
		case n <= 0:
			return 0
		case n > math.MaxInt64/int64(time.Second):
			// Beyond any push.max_age: the job ends the hint as expired.
			return time.Duration(math.MaxInt64)
		}
		return time.Duration(n) * time.Second
	}
	at, err := http.ParseTime(v)
	if err != nil {
		return 0
	}
	if d := at.Sub(now); d > 0 {
		return d
	}
	return 0
}
