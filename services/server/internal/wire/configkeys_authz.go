// SPDX-License-Identifier: Apache-2.0

package wire

import "github.com/OWNER/bothy/services/server/internal/platform/config"

// authzConfigKeys returns the keys the module authz reads: the bound of the rights snapshot cache
// (M1-28) and the three keys of the expiry jobs (M1-30) from docs/spec/07-permissions.md section 14,
// the age of a fresh step-up and the group text limits (M1-48) from 03-data-model.md
// "Configuration values of this chapter".
func authzConfigKeys() []config.Key {
	return []config.Key{
		{
			Name: "group.name_max_chars", Kind: config.KindInt, Default: "100", Min: "1",
			Doc:     "Longest group name in Unicode characters.",
			Chapter: chapter03,
		},
		{
			Name: "group.description_max_chars", Kind: config.KindInt, Default: "500", Min: "1",
			Doc:     "Longest group description in Unicode characters.",
			Chapter: chapter03,
		},
		{
			Name: "authz.snapshot_cache_max_entries", Kind: config.KindInt, Default: "10000", Min: "1",
			Doc:     "Most rights snapshots a process keeps in memory (least recently used go first).",
			Chapter: chapter07,
		},
		{
			// At least 1s: River's periodic jobs do not run more often in a useful way.
			Name: "authz.expiry_sweep_interval", Kind: config.KindDuration, Default: "1m", Min: "1s",
			Doc:     "Period of the jobs that report expired grants and memberships and send reminders before an expiry.",
			Chapter: chapter07,
		},
		{
			Name: "authz.expiry_reminder_lead", Kind: config.KindDuration, Default: "48h", Min: "1s",
			Doc:     "How long before an expiry the reminder access.expiring is written.",
			Chapter: chapter07,
		},
		{
			// The upper bound is the LIMIT parameter of the selections (int4).
			Name: "authz.expiry_sweep_batch_max", Kind: config.KindInt, Default: "500", Min: "1", Max: "2147483647",
			Doc:     "Rows per transaction of the jobs authz.expiry_sweep and authz.expiry_reminder.",
			Chapter: chapter07,
		},
		{
			// At least 1s: a shorter window would make every step-up stale before it is used.
			Name: "auth.step_up_max_age", Kind: config.KindDuration, Default: "5m", Min: "1s",
			Doc:     "How long a step-up confirmation counts as fresh for actions that need one.",
			Chapter: chapter03,
		},
	}
}
