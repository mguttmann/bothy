// SPDX-License-Identifier: Apache-2.0

// Package domain holds the pure rules of the agents module (M1-51): the agent record of
// docs/spec/03-datenmodell.md section 4.4, the backend policy with its transitions
// (docs/spec/10-backend-richtlinie.md section 9), the tool class order chat < scoped < host and
// the effective class (docs/spec/07-rechte.md section 2.11), the schema agent_operator_config.v1
// (docs/spec/09-agenten.md section 2.2) and the presence view. It knows no clock, database or
// other module.
//
// Validation errors are kernel/errs errors of code validation_failed with one field error whose
// pointer names the field of the request body; they never repeat the rejected value.
//
// Example:
//
//	cfg, err := domain.ParseOperatorConfig(map[string]any{"supports_voice": false})
//	a := domain.NewAgent(id, "hermes", "", cfg)
//	changed, err := a.SetPolicy(domain.Policy{Mode: domain.Personal, HolderID: owner}, owner, now)
package domain
