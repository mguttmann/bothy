// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"slices"
)

// SameReport reports two reports of session users as equal (Design note 8 of M2-40): the same set
// of os_user names, and for each name the same working directories in the same order. The order of
// the names does not matter; names compare exactly, case included (24 section 5.3).
func SameReport(a, b []ReportedSessionUser) bool {
	ma, mb := reportMap(a), reportMap(b)
	if len(ma) != len(mb) {
		return false
	}
	for name, dirs := range ma {
		other, ok := mb[name]
		if !ok || !slices.Equal(dirs, other) {
			return false
		}
	}
	return true
}

// VanishedNames returns the names of before that after no longer reports, compared exactly,
// ascending: their people lose the terminal on the host (07 SR12).
func VanishedNames(before, after []ReportedSessionUser) []string {
	now := reportMap(after)
	var out []string
	for name := range reportMap(before) {
		if _, ok := now[name]; !ok {
			out = append(out, name)
		}
	}
	slices.Sort(out)
	return out
}

// Reports reports whether list names osUser exactly, case included (the comparison of 24 section
// 5.3 and of session_user_not_on_host).
func Reports(list []ReportedSessionUser, osUser string) bool {
	return slices.ContainsFunc(list, func(u ReportedSessionUser) bool { return u.OSUser == osUser })
}

// reportMap indexes a report by name; a missing workdirs list is the empty list.
func reportMap(list []ReportedSessionUser) map[string][]string {
	out := make(map[string][]string, len(list))
	for _, u := range list {
		dirs := u.Workdirs
		if dirs == nil {
			dirs = []string{}
		}
		out[u.OSUser] = dirs
	}
	return out
}
