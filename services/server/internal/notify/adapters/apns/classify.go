// SPDX-License-Identifier: Apache-2.0

package apns

import (
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/notify/domain"
)

// The reasons of the APNs answers that 13 7.1 names [Q15].
const (
	reasonBadDeviceToken              = "BadDeviceToken"
	reasonDeviceTokenNotForTopic      = "DeviceTokenNotForTopic"
	reasonExpiredProviderToken        = "ExpiredProviderToken"
	reasonInvalidProviderToken        = "InvalidProviderToken"
	reasonMissingProviderToken        = "MissingProviderToken"
	reasonBadEnvironmentKeyIDInToken  = "BadEnvironmentKeyIdInToken"
	reasonUnrelatedKeyIDInToken       = "UnrelatedKeyIdInToken"
	reasonForbidden                   = "Forbidden"
	reasonTooManyProviderTokenUpdates = "TooManyProviderTokenUpdates"
	reasonTooManyRequests             = "TooManyRequests"
	reasonPayloadTooLarge             = "PayloadTooLarge"
	reasonUnregistered                = "Unregistered"
	reasonExpiredToken                = "ExpiredToken"
)

// tableReasons are the reasons that error.type may carry; any other reason is "other", so an
// answer of Apple never puts free text into a log record.
var tableReasons = []string{
	reasonBadDeviceToken, reasonDeviceTokenNotForTopic, reasonExpiredProviderToken, reasonInvalidProviderToken,
	reasonMissingProviderToken, reasonBadEnvironmentKeyIDInToken, reasonUnrelatedKeyIDInToken, reasonForbidden,
	reasonTooManyProviderTokenUpdates, reasonTooManyRequests, reasonPayloadTooLarge, reasonUnregistered, reasonExpiredToken,
}

// errorTypeOther is error.type of a reason outside the table.
const errorTypeOther = "other"

// errorType returns reason when it is one of the table, else "other".
func errorType(reason string) string {
	if slices.Contains(tableReasons, reason) {
		return reason
	}
	return errorTypeOther
}

// logLevel says which record an answer writes (Design note 7 of brief M3-20).
type logLevel int

const (
	logNone logLevel = iota
	logWarnBadToken
	logErrorConfig
	logErrorProgram
)

// verdict is the consequence of one APNs answer.
type verdict struct {
	// result is the SendResult for the job of M3-19.
	result domain.SendResult
	// renew asks for the one JWT replacement and retry of 403 ExpiredProviderToken.
	renew bool
	// pause starts the pause of the environment (a configuration error).
	pause bool
	// log is the record the answer writes; errType its error.type.
	log     logLevel
	errType string
}

// classify maps one answer onto the table of 13 7.1 with the unlisted rows of Design note 6 of
// brief M3-20: status decides the class, reason the detail; an unlisted reason follows the
// nearest listed answer of the same status. timestamp is the body member "timestamp" (zero when
// absent); retry5xx is push.apns.retry_5xx_delay. It is a pure function.
func classify(status int, reason string, timestamp time.Time, retry5xx time.Duration) verdict {
	program := verdict{result: domain.SendResult{Class: domain.ResultProgramError}, log: logErrorProgram, errType: errorType(reason)}
	switch {
	case status == 200:
		return verdict{result: domain.SendResult{Class: domain.ResultAccepted}}
	case status == 410:
		return verdict{result: domain.SendResult{Class: domain.ResultDisable, DisableReason: domain.DisableTokenInvalid, InvalidSince: timestamp}}
	case status == 400 && reason == reasonBadDeviceToken:
		return verdict{result: domain.SendResult{Class: domain.ResultDisable, DisableReason: domain.DisableTokenInvalid}, log: logWarnBadToken}
	case status == 400 && reason == reasonDeviceTokenNotForTopic:
		return verdict{result: domain.SendResult{Class: domain.ResultDisable, DisableReason: domain.DisableTopicMismatch}}
	case status == 403 && reason == reasonExpiredProviderToken:
		return verdict{renew: true}
	case status == 403:
		return verdict{result: domain.SendResult{Class: domain.ResultConfigError}, pause: true, log: logErrorConfig, errType: errorType(reason)}
	case status == 429 && reason == reasonTooManyProviderTokenUpdates:
		return program
	case status == 429:
		return verdict{result: domain.SendResult{Class: domain.ResultThrottled}}
	case status >= 400 && status < 500:
		return program
	case status >= 500 && status < 600:
		return verdict{result: domain.SendResult{Class: domain.ResultRetry, RetryAfter: retry5xx}}
	}
	return program
}
