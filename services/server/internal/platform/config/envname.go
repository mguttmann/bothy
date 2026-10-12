// SPDX-License-Identifier: Apache-2.0

package config

import (
	"regexp"
	"strings"
)

// envPrefix starts every environment variable of the server (02 "Code layout", product name).
const envPrefix = "BOTHY_"

// configFileVariable names the configuration file when Sources.File is empty. It is the only
// BOTHY_ variable that is no key (16-operations.md 5.1 item 3).
const configFileVariable = "BOTHY_CONFIG_FILE"

// segmentPattern is one segment of a key name. It excludes "__", leading and trailing "_" and
// upper case, which makes EnvName and KeyFromEnv exact inverses.
var segmentPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(_[a-z0-9]+)*$`)

// validName reports whether name is a key name: at least two segments joined by ".".
func validName(name string) bool {
	segments := strings.Split(name, ".")
	if len(segments) < 2 {
		return false
	}
	for _, s := range segments {
		if !segmentPattern.MatchString(s) {
			return false
		}
	}
	return true
}

// EnvName returns the environment variable of a key: "BOTHY_" plus the name in upper case with
// every "." replaced by "__" (16-operations.md 5.1 item 2). For a valid key name the
// result maps back with KeyFromEnv.
//
// Example: EnvName("sync.event_retention") is "BOTHY_SYNC__EVENT_RETENTION".
func EnvName(key string) string {
	return envPrefix + strings.ToUpper(strings.ReplaceAll(key, ".", "__"))
}

// KeyFromEnv returns the key name a variable stands for. ok is false unless variable is exactly
// EnvName of a valid key name; whether the schema knows that key is not checked here.
//
// Example: KeyFromEnv("BOTHY_A__B_C") is ("a.b_c", true); KeyFromEnv("BOTHY_A___B") is ("", false).
func KeyFromEnv(variable string) (key string, ok bool) {
	rest, found := strings.CutPrefix(variable, envPrefix)
	if !found {
		return "", false
	}
	key = strings.ToLower(strings.ReplaceAll(rest, "__", "."))
	if !validName(key) || EnvName(key) != variable {
		return "", false
	}
	return key, true
}
