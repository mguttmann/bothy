// SPDX-License-Identifier: Apache-2.0

package wire

import "github.com/OWNER/bothy/services/server/internal/platform/config"

// configKeyGroups lists the key groups of the configuration schema, one function per owning area
// in its own file configkeys_<area>.go. A ticket that builds the first consumer of a key adds the
// key to the file of its area, or a new file and one line here, and names the chapter with its
// constant from configchapters.go. Parallel tickets share this list, the list configChecks in
// configchecks.go, the constants in configchapters.go and the expected keys in
// TestConfigSchemaBuilds (02 "Configuration loading" item 5).
var configKeyGroups = []func() []config.Key{
	serverConfigKeys,
	httpConfigKeys,
	messagingConfigKeys,
	identityConfigKeys,
	apiConfigKeys,
	authzConfigKeys,
	chatConfigKeys,
	egressConfigKeys,
	secretsConfigKeys,
	signingKeyConfigKeys,
	databaseConfigKeys,
	migrateConfigKeys,
	backupConfigKeys,
	telemetryConfigKeys,
	shutdownConfigKeys,
	jobsConfigKeys,
	auditConfigKeys,
	rateLimitConfigKeys,
	filesConfigKeys,
	syncConfigKeys,
	gatewayConfigKeys,
	consumerConfigKeys,
	instanceConfigKeys,
	terminalConfigKeys,
	clusterConfigKeys,
	appsConfigKeys,
	searchConfigKeys,
	webhookConfigKeys,
	notifyConfigKeys,
	pushConfigKeys,
	agentsConfigKeys,
	runsConfigKeys,
	commandsConfigKeys,
	billingConfigKeys,
	statsConfigKeys,
	retentionConfigKeys,
	statusConfigKeys,
}

// ConfigSchema builds the configuration schema of the server: every key of configKeyGroups and
// every cross-check of configChecks. cmd/bothy loads the process configuration with it, and the
// roles bind their Config structs from the loaded values.
//
// The error reports an invalid key or check (see config.NewSchema and Schema.AddCheck); it is a
// programming error that the tests of this package catch.
func ConfigSchema() (*config.Schema, error) {
	return buildConfigSchema(configKeyGroups, configChecks)
}

// buildConfigSchema builds a schema from key groups and checks; tests pass invalid ones.
func buildConfigSchema(groups []func() []config.Key, checks []configCheck) (*config.Schema, error) {
	var keys []config.Key
	for _, group := range groups {
		keys = append(keys, group()...)
	}
	s, err := config.NewSchema(keys)
	if err != nil {
		return nil, err
	}
	for _, c := range checks {
		if err := s.AddCheck(c.name, c.keys, c.fn); err != nil {
			return nil, err
		}
	}
	return s, nil
}
