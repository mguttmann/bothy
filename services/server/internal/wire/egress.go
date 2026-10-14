// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"fmt"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
)

// egressValues are the keys egress.* as config.Bind fills them.
type egressValues struct {
	AllowedHosts     []string      `config:"egress.allowed_hosts"`
	Timeout          time.Duration `config:"egress.timeout"`
	MaxResponseBytes int64         `config:"egress.max_response_bytes"`
}

// EgressConfig builds the configuration of the egress client from the loaded values: every
// entry of egress.allowed_hosts parsed with egress.ParseRule, egress.timeout and
// egress.max_response_bytes. Each consumer passes it to egress.New with its own options.
//
// The error reports values that Load would have refused (the cross-check egress_allowed_hosts
// and the bounds of the keys); with values from Load it does not occur. It names the position
// of an entry, never its text.
func EgressConfig(v *config.Values) (egress.Config, error) {
	var raw egressValues
	if err := config.Bind(v, &raw); err != nil {
		return egress.Config{}, err
	}
	rules, err := egressRules(raw.AllowedHosts)
	if err != nil {
		return egress.Config{}, fmt.Errorf("config: egress.allowed_hosts: %w", err)
	}
	return egress.Config{AllowedHosts: rules, Timeout: raw.Timeout, MaxResponseBytes: raw.MaxResponseBytes}, nil
}

// egressRules parses the entries of egress.allowed_hosts and rejects two entries for the same
// host and port (19 "Configuration values of this chapter": duplicate entries are a
// load error). Errors name positions counted from 1, never an entry.
func egressRules(entries []string) ([]egress.Rule, error) {
	rules := make([]egress.Rule, 0, len(entries))
	seen := map[[2]any]int{}
	for i, e := range entries {
		r, err := egress.ParseRule(e)
		if err != nil {
			return nil, fmt.Errorf("element %d: %w", i+1, err)
		}
		key := [2]any{r.Host, r.Port}
		if j, dup := seen[key]; dup {
			return nil, fmt.Errorf("elements %d and %d name the same host and port", j+1, i+1)
		}
		seen[key] = i
		rules = append(rules, r)
	}
	return rules, nil
}

// egressAllowedHosts is the cross-check egress_allowed_hosts: it refuses an entry that
// egress.ParseRule refuses and two entries for the same host and port.
func egressAllowedHosts(v *config.Values) error {
	_, err := egressRules(v.Strings(keyEgressAllowedHosts))
	return err
}
