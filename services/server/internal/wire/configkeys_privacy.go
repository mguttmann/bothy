// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"github.com/OWNER/bothy/services/server/internal/kernel/personaldata"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
)

// privacyConfigKeys returns the keys of the privacy requests (docs/spec/20-privacy.md section 13,
// M7-06). The statutory deadline of a request is no key (R-DP-05).
func privacyConfigKeys() []config.Key {
	return []config.Key{
		{Name: "privacy.export_ttl", Kind: config.KindDuration, Default: "7d", Min: "1h", Chapter: chapter20,
			Doc: "How long an export package can be downloaded before it is deleted."},
		{Name: "privacy.export_requires_approval", Kind: config.KindBool, Default: "false", Chapter: chapter20,
			Doc: "Whether the export of one's own data needs the approval of an owner."},
		{Name: "privacy.account_deletion_content", Kind: config.KindEnum, Default: string(personaldata.KeepPseudonymized),
			Enum: []string{string(personaldata.KeepPseudonymized), string(personaldata.DeleteContent)}, Chapter: chapter20,
			Doc: "Contents of a deleted account: keep them under the pseudonym, or delete own messages and files."},
		{Name: "privacy.request_reminder_lead", Kind: config.KindDuration, Default: "7d", Min: "1h", Max: "27d", Chapter: chapter20,
			Doc: "How long before the statutory deadline of an open privacy request the owners are reminded."},
		{Name: "retention.privacy_job", Kind: config.KindDuration, Default: "1095d", Min: "1d", Chapter: chapter20,
			Doc: "How long finished privacy requests are kept (without export packages)."},
	}
}
