// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"errors"
	"fmt"
	"strings"

	"github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/instance/domain"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
)

// clientVersionKeys are the minimum and recommended app versions per platform of
// docs/spec/18-release.md section 9 item 2, in the order ios, android, macos, windows; discovery
// reports them (M1-46).
var clientVersionKeys = []string{
	"client.min_version.ios", "client.min_version.android", "client.min_version.macos", "client.min_version.windows",
	"client.recommended_version.ios", "client.recommended_version.android", "client.recommended_version.macos",
	"client.recommended_version.windows",
}

// instanceConfigKeys returns the keys owned by the module instance (M1-46): the length of the
// product name of the branding (03 section 9) and the app versions of 18 section 12. The version
// keys are strings, because the schema allows the empty default ("no bound") only for string
// kinds; the cross-check client_versions_semver requires SemVer without "v" when one is set.
func instanceConfigKeys() []config.Key {
	keys := []config.Key{{
		Name: "branding.product_name_max_chars", Kind: config.KindInt, Default: "64", Min: "1",
		Doc:     "Longest product name of the branding in Unicode characters.",
		Chapter: chapter03,
	}}
	for _, name := range clientVersionKeys {
		doc := "Lowest app version allowed to connect on this platform; empty means no bound."
		if strings.HasPrefix(name, "client.recommended_version.") {
			doc = "App version below which the app shows an update note on this platform; empty means none."
		}
		keys = append(keys, config.Key{Name: name, Kind: config.KindString, Default: "", Doc: doc, Chapter: chapter18})
	}
	return keys
}

// clientVersionsSemVer requires every set client version key to be a SemVer 2.0.0 version
// without a leading v (domain.ValidateSemVer); the error names the keys, never a value.
func clientVersionsSemVer(v *config.Values) error {
	var problems []error
	for _, key := range clientVersionKeys {
		if s := v.String(key); s != "" && domain.ValidateSemVer(s) != nil {
			problems = append(problems, fmt.Errorf("%s must be a semantic version (major.minor.patch after SemVer 2.0.0, without a leading v) or empty", key))
		}
	}
	return errors.Join(problems...)
}

// InstanceConfig binds the keys the settings, branding and discovery use cases of instance read
// (branding.product_name_max_chars, the client limits of getInstance and the client versions of
// discovery) into one typed value (M1-46). The error names the key whose binding failed, never
// a value.
func InstanceConfig(v *config.Values) (app.Config, error) {
	var c app.Config
	if err := config.Bind(v, &c); err != nil {
		return app.Config{}, err
	}
	return c, nil
}
