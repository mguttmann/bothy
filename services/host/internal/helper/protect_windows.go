// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// processSDDL gives SYSTEM and Administrators full access, the service account and OWNER RIGHTS
// only PROCESS_QUERY_LIMITED_INFORMATION (the service reads the token user for the peer check),
// which removes the implicit rights of the owner: no process of the session user can open the
// helper for writing (24 2.2 PTY helper point 4).
const processSDDL = "D:P(A;;GA;;;SY)(A;;GA;;;BA)(A;;0x1000;;;%s)(A;;0x1000;;;OW)"

// Protect replaces the DACL of the own process; serviceSID is the account of the host service.
func Protect(serviceSID string) error {
	sd, err := windows.SecurityDescriptorFromString(fmt.Sprintf(processSDDL, serviceSID))
	if err != nil {
		return fmt.Errorf("helper: %w", err)
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return fmt.Errorf("helper: %w", err)
	}
	return windows.SetSecurityInfo(windows.CurrentProcess(), windows.SE_KERNEL_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, dacl, nil)
}
