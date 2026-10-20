// SPDX-License-Identifier: Apache-2.0

package hostos

import (
	"context"
	"unsafe"

	"golang.org/x/sys/windows"
)

// SCM is the ServiceManager of Windows: QueryServiceStatusEx through the service control manager.
type SCM struct{}

// MainPID implements ServiceManager. It asks only for the rights a query needs
// (SC_MANAGER_CONNECT and SERVICE_QUERY_STATUS): the host service runs without administrator
// rights, and mgr.Connect and Mgr.OpenService would ask for full access, which Windows refuses it.
func (SCM) MainPID(_ context.Context, name string) (int, error) {
	m, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		return 0, err
	}
	defer func() { _ = windows.CloseServiceHandle(m) }()
	n, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return 0, ErrNotRunning
	}
	s, err := windows.OpenService(m, n, windows.SERVICE_QUERY_STATUS)
	if err != nil {
		return 0, ErrNotRunning
	}
	defer func() { _ = windows.CloseServiceHandle(s) }()
	var st windows.SERVICE_STATUS_PROCESS
	var needed uint32
	err = windows.QueryServiceStatusEx(s, windows.SC_STATUS_PROCESS_INFO, (*byte)(unsafe.Pointer(&st)), uint32(unsafe.Sizeof(st)), &needed) //nolint:gosec // documented signature
	if err != nil || st.ProcessId == 0 {
		return 0, ErrNotRunning
	}
	return int(st.ProcessId), nil
}
