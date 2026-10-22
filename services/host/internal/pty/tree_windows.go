// SPDX-License-Identifier: Apache-2.0

package pty

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// killedExitCode is the exit code TerminateJobObject gives every process of an ended tree.
const killedExitCode = 1

// platform is the Windows state of a Supervisor; the Job Objects belong to the sessions, whose
// handles the supervisor's process holds.
type platform struct{}

// prepare needs nothing on Windows.
func (*platform) prepare(Options) error { return nil }

// newJob creates the Job Object of one session: closing its last handle kills every process in
// it, and no process may leave it (no JOB_OBJECT_LIMIT_BREAKAWAY_OK), so
// CREATE_BREAKAWAY_FROM_JOB fails inside the session.
func newJob() (windows.Handle, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, err
	}
	var info windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	// The address stays inside the call expression, where the compiler keeps the structure alive
	// and in place.
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), //nolint:gosec // the API takes the address of the limit structure
		uint32(unsafe.Sizeof(info))); err != nil {
		_ = windows.CloseHandle(job)
		return 0, err
	}
	return job, nil
}
