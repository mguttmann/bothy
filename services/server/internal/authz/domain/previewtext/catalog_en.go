// SPDX-License-Identifier: Apache-2.0

package previewtext

// catalogEN is the English catalogue of the preview text (brief M1-49 "Catalog"; 07 section
// 9.3). Placeholders stand in braces; quotes are ASCII.
var catalogEN = catalog{
	"level.read":                          "Read",
	"level.write":                         "Write",
	"level.trigger":                       "Trigger",
	"subject.group":                       "Group {group}",
	"where.instance":                      "in the whole instance",
	"where.space":                         "in space {space}",
	"where.channel":                       "in channel {space}/{channel}",
	"summary":                             `{subject} gets the level "{level}" {where}{until}.`,
	"until":                               " until {date}",
	"abilities.single":                    "{subject} can then: {abilities}.",
	"abilities.group":                     "Members of {group} can then: {abilities}.",
	"ability.read":                        "read messages",
	"ability.write":                       "read and write messages, create threads",
	"ability.trigger_agent":               "trigger agent {agent}",
	"caps":                                "Upload files: {upload}. Confirm approval cards: {approve}.",
	"caps.export":                         "Export: yes.",
	"caps.manage":                         "Manage: yes.",
	keyYes:                                "yes",
	keyNo:                                 "no",
	"denial.delegation_exceeds_own_level": "Rejected: you can grant at most your own level.",
	"denial.delegation_exceeds_own_capabilities": "Rejected: you can only grant additional rights you have yourself.",
	"denial.subject_kind_not_allowed":            "Rejected: agents and apps get grants only from an owner, at most Write and only Upload files.",
	"denial.owner_only":                          "Rejected: only an owner grants on the whole instance.",
	"hint.i1":                                    "Rejected: {agent} runs on the personal subscription of {holder}. Switch it to organization first.",
	"hint.tool_class":                            "Note: {agent} has the tool class host; {subject} cannot trigger {agent} (owners only).",
	"hint.read_horizon":                          "Note: {agent} also reads {space}/{channel}. {subject} may not read {space}/{channel} but can learn its contents through {agent}.",
	"hint.read_horizon.group":                    "Note: {agent} also reads {space}/{channel}. {count} members of {group} may not read {space}/{channel} but can learn its contents through {agent}.",
	"hint.read_horizon.group_one":                "Note: {agent} also reads {space}/{channel}. 1 member of {group} may not read {space}/{channel} but can learn its contents through {agent}.",
	"hint.group_reach":                           "Note: the group {group} reaches {count} people.",
	"hint.group_reach.one":                       "Note: the group {group} reaches one person.",
	"hint.expiry_missing":                        "Note: the grant for {subject} has no expiry date.",
	"date":                                       "2006-01-02",
}
