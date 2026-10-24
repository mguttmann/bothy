// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"slices"
	"strconv"
	"strings"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// The JSON pointers of the fields of PushRegistrationCreate in a validation error.
const (
	pointerProvider    = "/provider"
	pointerEnvironment = "/environment"
	pointerToken       = "/token"
	pointerAppID       = "/app_id"
)

// The providers of push_registrations.provider (03, 13 section 5).
const (
	// ProviderAPNs is Apple Push Notification service (iOS and macOS, 13 5.1).
	ProviderAPNs = "apns"
	// ProviderFCM is Firebase Cloud Messaging (Android, 13 5.2).
	ProviderFCM = "fcm"
	// ProviderUnifiedPush and ProviderWNS have no adapter in this server version (13 5.4).
	ProviderUnifiedPush = "unifiedpush"
	ProviderWNS         = "wns"
)

// The environments of push_registrations.environment (03); only APNs has two (13 5.1).
const (
	// EnvironmentProduction is the default when the request names none.
	EnvironmentProduction = "production"
	// EnvironmentSandbox is the APNs development environment.
	EnvironmentSandbox = "sandbox"
)

// The field codes of a refused registration (brief M3-22 design note 4; 06 section 5 point 3).
const (
	// FieldNotEnabled marks a provider without an adapter in this server version.
	FieldNotEnabled = "not_enabled"
	// FieldInvalid marks a value that does not fit the device, the provider or the topics.
	FieldInvalid = "invalid"
	// FieldPatternMismatch marks a token outside the form of its provider.
	FieldPatternMismatch = "pattern_mismatch"
	// FieldTooLong marks a token or app_id above push.registration_field_max_bytes.
	FieldTooLong = "too_long"
)

// ReasonPushProviderDisabled is the reason of 403 feature_disabled for a provider that is not
// enabled on this instance (13 section 13, docs/DECISIONS.md D-M3-22-3).
const ReasonPushProviderDisabled = "push_provider_disabled"

// RegistrationInput is the body of registerPush with the platform of the caller's device.
type RegistrationInput struct {
	// Provider, Token, Environment and AppID are the members of PushRegistrationCreate;
	// Environment is "" when the body leaves it out.
	Provider, Token, Environment, AppID string
	// Platform is devices.platform of the caller's own device.
	Platform string
}

// RegistrationPolicy are the provider facts of this instance (internal/wire builds them from
// apns.Config.Enabled, fcm.Config.Enabled, push.apns.topics and
// push.registration_field_max_bytes).
type RegistrationPolicy struct {
	// Enabled holds the providers with a configured adapter (apns, fcm).
	Enabled map[string]bool
	// APNsTopics are the allowed bundle IDs of push.apns.topics.
	APNsTopics []string
	// FieldMaxBytes is push.registration_field_max_bytes, the largest token and app_id.
	FieldMaxBytes int
}

// ValidateRegistration checks in of registerPush against pol in the order of brief M3-22 design
// note 4 and returns the registration as it is stored: the environment production when absent,
// an apns token in lower case. The first two steps end the check, the field rules are collected.
//
//  1. Provider unifiedpush or wns: validation_failed with not_enabled at /provider.
//  2. Provider apns or fcm that pol does not enable: feature_disabled with the reason
//     push_provider_disabled.
//  3. Field rules: the platform of the device fits the provider (invalid at /provider); sandbox
//     only with apns (invalid at /environment); an apns token is hexadecimal in an even number of
//     digits, an fcm token printable ASCII without white space (pattern_mismatch at /token); token
//     and app_id at most FieldMaxBytes bytes (too_long); an apns app_id is one of APNsTopics
//     (invalid at /app_id).
//
// Errors: *errs.Error as above; nothing else.
//
// Example:
//
//	reg, err := domain.ValidateRegistration(domain.RegistrationInput{Provider: "apns", Token: hex, AppID: "com.example.chat", Platform: "ios"}, pol)
func ValidateRegistration(in RegistrationInput, pol RegistrationPolicy) (RegistrationInput, error) {
	switch in.Provider {
	case ProviderUnifiedPush, ProviderWNS:
		return RegistrationInput{}, errs.New(errs.CodeValidationFailed, "the push registration is invalid").WithFields(
			errs.FieldError{Pointer: pointerProvider, Code: FieldNotEnabled, Detail: "this server version has no adapter for the provider"})
	case ProviderAPNs, ProviderFCM:
		if !pol.Enabled[in.Provider] {
			return RegistrationInput{}, errs.New(errs.CodeFeatureDisabled, "the push provider is not enabled on this instance").
				WithReason(ReasonPushProviderDisabled)
		}
	default:
		// The schema PushProvider refuses other values before the use case; this guard keeps the
		// rule closed for a caller outside the HTTP chain.
		return RegistrationInput{}, errs.New(errs.CodeValidationFailed, "the push registration is invalid").WithFields(
			errs.FieldError{Pointer: pointerProvider, Code: FieldInvalid, Detail: "unknown provider"})
	}
	out := in
	if out.Environment == "" {
		out.Environment = EnvironmentProduction
	}
	fields := registrationFields(&out, pol)
	if len(fields) > 0 {
		return RegistrationInput{}, errs.New(errs.CodeValidationFailed, "the push registration is invalid").WithFields(fields...)
	}
	return out, nil
}

// registrationFields collects the field rules of step 3 and lowers an apns token in place.
func registrationFields(in *RegistrationInput, pol RegistrationPolicy) []errs.FieldError {
	var fields []errs.FieldError
	apns := in.Provider == ProviderAPNs
	p := Platform(in.Platform)
	if apns && p != PlatformIOS && p != PlatformMacOS || !apns && p != PlatformAndroid {
		fields = append(fields, errs.FieldError{Pointer: pointerProvider, Code: FieldInvalid, Detail: "the provider does not serve the platform of this device"})
	}
	switch in.Environment {
	case EnvironmentProduction:
	case EnvironmentSandbox:
		if !apns {
			fields = append(fields, errs.FieldError{Pointer: pointerEnvironment, Code: FieldInvalid, Detail: "only apns has a sandbox environment"})
		}
	default:
		fields = append(fields, errs.FieldError{Pointer: pointerEnvironment, Code: FieldInvalid, Detail: "unknown environment"})
	}
	maxBytes := strconv.Itoa(pol.FieldMaxBytes)
	switch {
	case len(in.Token) > pol.FieldMaxBytes:
		fields = append(fields, errs.FieldError{Pointer: pointerToken, Code: FieldTooLong, Detail: "the token has more than " + maxBytes + " bytes"})
	case apns && !hexToken(in.Token):
		fields = append(fields, errs.FieldError{Pointer: pointerToken, Code: FieldPatternMismatch, Detail: "an apns token is an even number of hexadecimal digits"})
	case !apns && !printableToken(in.Token):
		fields = append(fields, errs.FieldError{Pointer: pointerToken, Code: FieldPatternMismatch, Detail: "an fcm token is printable ASCII without white space"})
	}
	if apns {
		in.Token = strings.ToLower(in.Token)
	}
	switch {
	case len(in.AppID) > pol.FieldMaxBytes:
		fields = append(fields, errs.FieldError{Pointer: pointerAppID, Code: FieldTooLong, Detail: "the app_id has more than " + maxBytes + " bytes"})
	case apns && !slices.Contains(pol.APNsTopics, in.AppID):
		fields = append(fields, errs.FieldError{Pointer: pointerAppID, Code: FieldInvalid, Detail: "the app_id is not an allowed topic of this instance"})
	}
	return fields
}

// hexToken reports a non-empty even number of hexadecimal digits (the form M3-20 sends in the
// request path).
func hexToken(s string) bool {
	if s == "" || len(s)%2 != 0 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}

// printableToken reports a non-empty string of printable ASCII without white space (0x21 to
// 0x7e).
func printableToken(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < 0x21 || s[i] > 0x7e {
			return false
		}
	}
	return true
}

// The values of push_registrations.disabled_reason this ticket writes (03; 13 6.4 and 7.3).
const (
	// DisabledLogout ends a registration whose session ended.
	DisabledLogout = "logout"
	// DisabledDeviceRevoked ends a registration of a revoked device or a disabled person.
	DisabledDeviceRevoked = "device_revoked"
	// DisabledTokenInvalid ends the older registration of the same app installation (brief M3-22
	// design note 5 d).
	DisabledTokenInvalid = "token_invalid"
)

// DisableReasonForSession maps the revocation reason of auth_sessions onto
// push_registrations.disabled_reason (brief M3-22 design note 7 a, docs/DECISIONS.md
// D-M3-22-7): device_revoked and user_disabled give device_revoked; every other reason (logout,
// admin, refresh_reuse, inactive and any later one) gives logout, because for a registration
// "its session ended" is what logout stands for in 13 6.4.
func DisableReasonForSession(reason string) string {
	switch reason {
	case "device_revoked", "user_disabled":
		return DisabledDeviceRevoked
	}
	return DisabledLogout
}
