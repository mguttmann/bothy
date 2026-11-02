// SPDX-License-Identifier: Apache-2.0

package domain

import "encoding/json"

// Profile is one entry of hosts.reported_profiles (schema host_profiles.v1): a name, a display
// name and a kind (shell, claude or command); never a command line, a path or an environment.
type Profile struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Kind        string `json:"kind"`
}

// ReportedSessionUser is one entry of hosts.reported_session_users (schema
// host_reported_session_users.v1): a name of host_service.session_users with the display names of
// its working directories of the profile claude; never a path. OwnerAccount marks the account of
// the owner account mode (ADR 0073), absent otherwise.
type ReportedSessionUser struct {
	OSUser       string   `json:"os_user"`
	Workdirs     []string `json:"workdirs"`
	OwnerAccount bool     `json:"owner_account,omitempty"`
}

// DoctorCheck is one entry of hosts.reported_doctor (schema host_doctor.v1): the result and code
// of a check HD1 to HD11 and, after the rule for names of 24 section 2.2, a session user name.
type DoctorCheck struct {
	Check       string `json:"check"`
	OK          bool   `json:"ok"`
	Code        string `json:"code,omitempty"`
	SessionUser string `json:"session_user,omitempty"`
}

// DecodeProfiles decodes reported_profiles; nil, a non-list or broken JSON is the empty list (the
// host service is the writer, M2-41 and M2-43 validate before they store).
func DecodeProfiles(raw []byte) []Profile { return decodeList[Profile](raw) }

// DecodeReportedSessionUsers decodes reported_session_users like DecodeProfiles; a missing
// workdirs member is the empty list.
func DecodeReportedSessionUsers(raw []byte) []ReportedSessionUser {
	out := decodeList[ReportedSessionUser](raw)
	for i := range out {
		if out[i].Workdirs == nil {
			out[i].Workdirs = []string{}
		}
	}
	return out
}

// DecodeDoctor decodes reported_doctor like DecodeProfiles.
func DecodeDoctor(raw []byte) []DoctorCheck { return decodeList[DoctorCheck](raw) }

// decodeList decodes a JSON list; anything else is the empty list.
func decodeList[T any](raw []byte) []T {
	var out []T
	if err := json.Unmarshal(raw, &out); err != nil || out == nil {
		return []T{}
	}
	return out
}

// ReportedNames returns the os_user names of a reported list.
func ReportedNames(list []ReportedSessionUser) []string {
	out := make([]string, 0, len(list))
	for _, u := range list {
		out = append(out, u.OSUser)
	}
	return out
}
